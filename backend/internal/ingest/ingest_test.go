package ingest_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/ingest"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// s3Event builds an object-created event for one key.
func s3Event(key string) events.S3Event {
	return s3EventSized(key, 0)
}

// s3EventSized builds an object-created event carrying the object's real size, as S3 reports it.
func s3EventSized(key string, size int64) events.S3Event {
	return events.S3Event{Records: []events.S3EventRecord{
		{S3: events.S3Entity{Object: events.S3Object{Key: key, Size: size}}},
	}}
}

// hashOf returns the SHA-256 hex of content, the id a file settles under.
func hashOf(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func TestSettleTranscribesAnImageIntoTheKBSource(t *testing.T) {
	// Arrange: an image, which the Knowledge Base cannot read, is transcribed to searchable text.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	transcriber := mocks.NewMockTranscriber(ctrl)

	content := []byte("the image bytes")
	hash := hashOf(content)
	staging := "uploads/upl-1"
	raw := "files/" + hash
	kbSource := "kb/" + hash
	kbMeta := kbSource + ".metadata.json"
	pending := domain.File{ID: "upl-1", OwnerID: "alice", Key: staging, Name: "passport.jpg", ContentType: "image/jpeg", Status: domain.StatusPending}

	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(pending, nil)
	blobs.EXPECT().Get(gomock.Any(), staging).Return(content, "image/jpeg", nil)
	blobs.EXPECT().Copy(gomock.Any(), staging, raw).Return(nil)
	transcriber.EXPECT().Transcribe(gomock.Any(), content, "image/jpeg").Return("Passport no. RA3495037", nil)
	blobs.EXPECT().Put(gomock.Any(), kbSource, "text/plain; charset=utf-8", []byte("Passport no. RA3495037")).Return(nil)
	var metaBody []byte
	blobs.EXPECT().Put(gomock.Any(), kbMeta, "application/json", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, _ string, body []byte) error {
		metaBody = body
		return nil
	})
	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})
	idx.EXPECT().Delete(gomock.Any(), "upl-1").Return(nil)
	blobs.EXPECT().Delete(gomock.Any(), staging).Return(nil)

	h := ingest.NewHandler(idx, blobs, transcriber)

	// Act
	err := h.Handle(context.Background(), s3Event(staging))

	// Assert: settled under the hash as landed, raw kept for download, transcript is the KB source.
	require.NoError(t, err)
	assert.Equal(t, hash, saved.ID)
	assert.Equal(t, raw, saved.Key)
	assert.Equal(t, domain.StatusLanded, saved.Status)

	// The sidecar next to the KB source ties indexed passages back to this file.
	var meta struct {
		MetadataAttributes map[string]string `json:"metadataAttributes"`
	}
	require.NoError(t, json.Unmarshal(metaBody, &meta))
	assert.Equal(t, hash, meta.MetadataAttributes["fileId"])
	assert.Equal(t, "passport.jpg", meta.MetadataAttributes["fileName"])
}

func TestSettleSniffsAMislabelledImageAndTranscribesIt(t *testing.T) {
	// Arrange: a real image uploaded as application/octet-stream, as browsers do for HEIC. It must
	// be detected as an image and transcribed, not copied straight to the KB where it would fail.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	transcriber := mocks.NewMockTranscriber(ctrl)

	var pngBuf bytes.Buffer
	require.NoError(t, png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 8, 8))))
	content := pngBuf.Bytes()
	hash := hashOf(content)
	staging := "uploads/upl-1"
	pending := domain.File{ID: "upl-1", OwnerID: "alice", Key: staging, Name: "IMG.HEIC", ContentType: "application/octet-stream", Status: domain.StatusPending}

	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(pending, nil)
	blobs.EXPECT().Get(gomock.Any(), staging).Return(content, "application/octet-stream", nil)
	blobs.EXPECT().Copy(gomock.Any(), staging, "files/"+hash).Return(nil)
	// Detected as an image, so it is transcribed (the sniffed type here is image/png) not copied.
	transcriber.EXPECT().Transcribe(gomock.Any(), content, "image/png").Return("some text", nil)
	blobs.EXPECT().Put(gomock.Any(), "kb/"+hash, "text/plain; charset=utf-8", []byte("some text")).Return(nil)
	blobs.EXPECT().Put(gomock.Any(), "kb/"+hash+".metadata.json", "application/json", gomock.Any()).Return(nil)
	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})
	idx.EXPECT().Delete(gomock.Any(), "upl-1").Return(nil)
	blobs.EXPECT().Delete(gomock.Any(), staging).Return(nil)

	h := ingest.NewHandler(idx, blobs, transcriber)

	// Act & Assert: routed to transcription, and the record's content type is corrected.
	require.NoError(t, h.Handle(context.Background(), s3Event(staging)))
	assert.Equal(t, "image/png", saved.ContentType)
}

func TestSettleCopiesAParseableDocumentAsTheKBSource(t *testing.T) {
	// Arrange: a document the Knowledge Base parses itself is copied through, not transcribed.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	transcriber := mocks.NewMockTranscriber(ctrl)

	content := []byte("plain document text")
	hash := hashOf(content)
	staging := "uploads/upl-1"
	raw := "files/" + hash
	kbSource := "kb/" + hash
	pending := domain.File{ID: "upl-1", OwnerID: "alice", Key: staging, Name: "notes.txt", ContentType: "text/plain", Status: domain.StatusPending}

	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(pending, nil)
	blobs.EXPECT().Get(gomock.Any(), staging).Return(content, "text/plain", nil)
	blobs.EXPECT().Copy(gomock.Any(), staging, raw).Return(nil)
	// Not an image or PDF, so the raw is copied to the KB source and the transcriber is never called.
	blobs.EXPECT().Copy(gomock.Any(), raw, kbSource).Return(nil)
	blobs.EXPECT().Put(gomock.Any(), kbSource+".metadata.json", "application/json", gomock.Any()).Return(nil)
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).Return(nil)
	idx.EXPECT().Delete(gomock.Any(), "upl-1").Return(nil)
	blobs.EXPECT().Delete(gomock.Any(), staging).Return(nil)

	h := ingest.NewHandler(idx, blobs, transcriber)

	// Act & Assert
	require.NoError(t, h.Handle(context.Background(), s3Event(staging)))
}

func TestSettleTranscriptionErrorIsReturned(t *testing.T) {
	// Arrange: transcription fails, so the invocation fails and the S3 event redrives.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	transcriber := mocks.NewMockTranscriber(ctrl)

	content := []byte("the image bytes")
	staging := "uploads/upl-1"
	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(domain.File{ID: "upl-1", Key: staging, Name: "x.jpg", ContentType: "image/jpeg", Status: domain.StatusPending}, nil)
	blobs.EXPECT().Get(gomock.Any(), staging).Return(content, "image/jpeg", nil)
	blobs.EXPECT().Copy(gomock.Any(), staging, gomock.Any()).Return(nil)
	transcriber.EXPECT().Transcribe(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("model down"))

	h := ingest.NewHandler(idx, blobs, transcriber)

	// Act & Assert: nothing settled, the record stays pending for the redrive.
	assert.Error(t, h.Handle(context.Background(), s3Event(staging)))
}

func TestSettleReadErrorIsReturned(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)

	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(domain.File{ID: "upl-1", Key: "uploads/upl-1", Status: domain.StatusPending}, nil)
	blobs.EXPECT().Get(gomock.Any(), "uploads/upl-1").Return(nil, "", errors.New("s3 down"))

	h := ingest.NewHandler(idx, blobs, mocks.NewMockTranscriber(ctrl))

	// Act & Assert
	assert.Error(t, h.Handle(context.Background(), s3Event("uploads/upl-1")))
}

func TestSettleAlreadySettledIsNoOp(t *testing.T) {
	// Arrange: the pending record is gone, so this is a redelivered event for a settled file.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(domain.File{}, index.ErrNotFound)

	h := ingest.NewHandler(idx, mocks.NewMockStore(ctrl), mocks.NewMockTranscriber(ctrl))

	// Act & Assert: no-op, no error, nothing else touched.
	require.NoError(t, h.Handle(context.Background(), s3Event("uploads/upl-1")))
}

func TestSettleGetErrorIsReturned(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(domain.File{}, errors.New("dynamo down"))

	h := ingest.NewHandler(idx, mocks.NewMockStore(ctrl), mocks.NewMockTranscriber(ctrl))

	// Act & Assert
	assert.Error(t, h.Handle(context.Background(), s3Event("uploads/upl-1")))
}
