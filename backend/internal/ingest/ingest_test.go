package ingest_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

func TestSettleMovesToContentKeyAndWritesMetadata(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)

	content := []byte("the file bytes")
	hash := hashOf(content)
	staging := "uploads/upl-1"
	canonical := "files/" + hash
	metaKey := canonical + ".metadata.json"
	pending := domain.File{ID: "upl-1", OwnerID: "alice", Key: staging, Name: "petrol.jpg", ContentType: "image/jpeg", Status: domain.StatusPending}

	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(pending, nil)
	blobs.EXPECT().Get(gomock.Any(), staging).Return(content, "image/jpeg", nil)
	blobs.EXPECT().Copy(gomock.Any(), staging, canonical).Return(nil)
	var metaBody []byte
	blobs.EXPECT().Put(gomock.Any(), metaKey, "application/json", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, _ string, body []byte) error {
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

	h := ingest.NewHandler(idx, blobs)

	// Act
	err := h.Handle(context.Background(), s3Event(staging))

	// Assert: the record settled under the content hash as landed, keeping its name.
	require.NoError(t, err)
	assert.Equal(t, hash, saved.ID)
	assert.Equal(t, canonical, saved.Key)
	assert.Equal(t, domain.StatusLanded, saved.Status)
	assert.Equal(t, "petrol.jpg", saved.Name)

	// The metadata sidecar ties the file id and name to every passage the Knowledge Base indexes.
	var meta struct {
		MetadataAttributes map[string]string `json:"metadataAttributes"`
	}
	require.NoError(t, json.Unmarshal(metaBody, &meta))
	assert.Equal(t, hash, meta.MetadataAttributes["fileId"])
	assert.Equal(t, "petrol.jpg", meta.MetadataAttributes["fileName"])
}

func TestSettleMetadataWriteErrorIsReturned(t *testing.T) {
	// Arrange: the sidecar write fails, so the invocation fails and the S3 event redrives.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)

	content := []byte("the file bytes")
	staging := "uploads/upl-1"
	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(domain.File{ID: "upl-1", Key: staging, Name: "x", Status: domain.StatusPending}, nil)
	blobs.EXPECT().Get(gomock.Any(), staging).Return(content, "image/jpeg", nil)
	blobs.EXPECT().Copy(gomock.Any(), staging, gomock.Any()).Return(nil)
	blobs.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("s3 down"))

	h := ingest.NewHandler(idx, blobs)

	// Act
	err := h.Handle(context.Background(), s3Event(staging))

	// Assert: nothing settled, the record stays pending for the redrive.
	assert.Error(t, err)
}

func TestSettleReadErrorIsReturned(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)

	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(domain.File{ID: "upl-1", Key: "uploads/upl-1", Status: domain.StatusPending}, nil)
	blobs.EXPECT().Get(gomock.Any(), "uploads/upl-1").Return(nil, "", errors.New("s3 down"))

	h := ingest.NewHandler(idx, blobs)

	// Act
	err := h.Handle(context.Background(), s3Event("uploads/upl-1"))

	// Assert
	assert.Error(t, err)
}

func TestSettleAlreadySettledIsNoOp(t *testing.T) {
	// Arrange: the pending record is gone, so this is a redelivered event for a settled file.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(domain.File{}, index.ErrNotFound)

	h := ingest.NewHandler(idx, mocks.NewMockStore(ctrl))

	// Act
	err := h.Handle(context.Background(), s3Event("uploads/upl-1"))

	// Assert: no-op, no error, nothing else touched.
	require.NoError(t, err)
}

func TestSettleGetErrorIsReturned(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(domain.File{}, errors.New("dynamo down"))

	h := ingest.NewHandler(idx, mocks.NewMockStore(ctrl))

	// Act
	err := h.Handle(context.Background(), s3Event("uploads/upl-1"))

	// Assert
	assert.Error(t, err)
}
