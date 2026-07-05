package ingest_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/ingest"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// s3Event builds an object-created event for one key.
func s3Event(key string) events.S3Event {
	return events.S3Event{Records: []events.S3EventRecord{
		{S3: events.S3Entity{Object: events.S3Object{Key: key}}},
	}}
}

// hashOf returns the SHA-256 hex of content, the form used for a file id.
func hashOf(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func TestHandleExtractionSucceeds(t *testing.T) {
	// Arrange: the record id is the content hash, so verification passes.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	extractor := mocks.NewMockExtractor(ctrl)

	content := []byte("the file bytes")
	id := hashOf(content)
	key := "files/" + id
	stored := domain.File{ID: id, Key: key, Status: domain.StatusPending, Meta: map[string]string{"note": "keep"}}
	idx.EXPECT().Get(gomock.Any(), id).Return(stored, nil)
	blobs.EXPECT().Get(gomock.Any(), key).Return(content, "image/jpeg", nil)
	extractor.EXPECT().Extract(gomock.Any(), content, "image/jpeg").
		Return(map[string]string{"vendor": "Shell", "amount": "52.30"}, nil)

	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})

	embedder := mocks.NewMockEmbedder(ctrl)
	store := mocks.NewMockVectorStore(ctrl)
	embedder.EXPECT().Embed(gomock.Any(), gomock.Any()).Return([]float32{0.1, 0.2}, nil)
	store.EXPECT().Put(gomock.Any(), id, []float32{0.1, 0.2}).Return(nil)

	h := ingest.New(idx, blobs, extractor, embedder, store)

	// Act
	err := h.Handle(context.Background(), s3Event(key))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, domain.StatusReady, saved.Status)
	assert.Equal(t, "Shell", saved.Meta["vendor"])
	assert.Equal(t, "keep", saved.Meta["note"])
}

func TestHandleSkipsAlreadyReady(t *testing.T) {
	// Arrange: a redelivered event for a file already extracted; nothing should re-run.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	extractor := mocks.NewMockExtractor(ctrl)
	idx.EXPECT().Get(gomock.Any(), "abc").Return(domain.File{ID: "abc", Key: "files/abc", Status: domain.StatusReady}, nil)

	h := ingest.New(idx, blobs, extractor, mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

	// Act
	err := h.Handle(context.Background(), s3Event("files/abc"))

	// Assert: no blobs.Get, extract, or Put expected.
	require.NoError(t, err)
}

func TestHandleHashMismatchMarksFailed(t *testing.T) {
	// Arrange: the stored bytes do not hash to the id, so the upload is not what was registered.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	extractor := mocks.NewMockExtractor(ctrl)
	stored := domain.File{ID: "wronghash", Key: "files/wronghash", Status: domain.StatusPending}
	idx.EXPECT().Get(gomock.Any(), "wronghash").Return(stored, nil)
	blobs.EXPECT().Get(gomock.Any(), "files/wronghash").Return([]byte("mismatched bytes"), "image/jpeg", nil)

	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})

	h := ingest.New(idx, blobs, extractor, mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

	// Act: extractor is never called.
	err := h.Handle(context.Background(), s3Event("files/wronghash"))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, saved.Status)
}

func TestHandleExtractionFailsMarksFailed(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	extractor := mocks.NewMockExtractor(ctrl)

	content := []byte("the file bytes")
	id := hashOf(content)
	key := "files/" + id
	idx.EXPECT().Get(gomock.Any(), id).Return(domain.File{ID: id, Key: key, Status: domain.StatusPending}, nil)
	blobs.EXPECT().Get(gomock.Any(), key).Return(content, "image/jpeg", nil)
	extractor.EXPECT().Extract(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("model down"))

	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})

	h := ingest.New(idx, blobs, extractor, mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

	// Act
	err := h.Handle(context.Background(), s3Event(key))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, saved.Status)
}

func TestHandleReadErrorIsReturned(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	extractor := mocks.NewMockExtractor(ctrl)

	idx.EXPECT().Get(gomock.Any(), "abc").Return(domain.File{ID: "abc", Key: "files/abc"}, nil)
	blobs.EXPECT().Get(gomock.Any(), "files/abc").Return(nil, "", errors.New("s3 down"))

	h := ingest.New(idx, blobs, extractor, mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

	// Act
	err := h.Handle(context.Background(), s3Event("files/abc"))

	// Assert
	assert.Error(t, err)
}
