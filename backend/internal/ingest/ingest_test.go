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
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/ingest"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// s3Event builds an object-created event for one key.
func s3Event(key string) events.S3Event {
	return events.S3Event{Records: []events.S3EventRecord{
		{S3: events.S3Entity{Object: events.S3Object{Key: key}}},
	}}
}

// hashOf returns the SHA-256 hex of content, the id a file settles under.
func hashOf(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func TestSettleMovesToContentKeyAndExtracts(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	extractor := mocks.NewMockExtractor(ctrl)
	embedder := mocks.NewMockEmbedder(ctrl)
	store := mocks.NewMockVectorStore(ctrl)

	content := []byte("the file bytes")
	hash := hashOf(content)
	staging := "uploads/upl-1"
	canonical := "files/" + hash
	pending := domain.File{ID: "upl-1", OwnerID: "alice", Key: staging, Name: "petrol.jpg", Status: domain.StatusPending, Meta: map[string]string{"note": "keep"}}

	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(pending, nil)
	blobs.EXPECT().Get(gomock.Any(), staging).Return(content, "image/jpeg", nil)
	blobs.EXPECT().Copy(gomock.Any(), staging, canonical).Return(nil)
	extractor.EXPECT().Extract(gomock.Any(), content, "image/jpeg").Return(map[string]string{"vendor": "Shell"}, nil)
	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})
	embedder.EXPECT().Embed(gomock.Any(), gomock.Any()).Return([]float32{0.1}, nil)
	store.EXPECT().Put(gomock.Any(), hash, "alice", []float32{0.1}).Return(nil)
	idx.EXPECT().Delete(gomock.Any(), "upl-1").Return(nil)
	blobs.EXPECT().Delete(gomock.Any(), staging).Return(nil)

	h := ingest.New(idx, blobs, extractor, embedder, store)

	// Act
	err := h.Handle(context.Background(), s3Event(staging))

	// Assert: the record settled under the content hash, keeping the name and metadata.
	require.NoError(t, err)
	assert.Equal(t, hash, saved.ID)
	assert.Equal(t, canonical, saved.Key)
	assert.Equal(t, domain.StatusReady, saved.Status)
	assert.Equal(t, "petrol.jpg", saved.Name)
	assert.Equal(t, "Shell", saved.Meta["vendor"])
	assert.Equal(t, "keep", saved.Meta["note"])
}

func TestSettleExtractionFailsMarksFailedAndCleansUp(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	extractor := mocks.NewMockExtractor(ctrl)

	content := []byte("the file bytes")
	hash := hashOf(content)
	staging := "uploads/upl-1"
	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(domain.File{ID: "upl-1", Key: staging, Status: domain.StatusPending}, nil)
	blobs.EXPECT().Get(gomock.Any(), staging).Return(content, "image/jpeg", nil)
	blobs.EXPECT().Copy(gomock.Any(), staging, "files/"+hash).Return(nil)
	extractor.EXPECT().Extract(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("model down"))
	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})
	idx.EXPECT().Delete(gomock.Any(), "upl-1").Return(nil)
	blobs.EXPECT().Delete(gomock.Any(), staging).Return(nil)

	h := ingest.New(idx, blobs, extractor, mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

	// Act
	err := h.Handle(context.Background(), s3Event(staging))

	// Assert: still settled under the hash, marked failed, staging cleaned up.
	require.NoError(t, err)
	assert.Equal(t, hash, saved.ID)
	assert.Equal(t, domain.StatusFailed, saved.Status)
}

func TestSettleReadErrorIsReturned(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)

	idx.EXPECT().Get(gomock.Any(), "upl-1").Return(domain.File{ID: "upl-1", Key: "uploads/upl-1"}, nil)
	blobs.EXPECT().Get(gomock.Any(), "uploads/upl-1").Return(nil, "", errors.New("s3 down"))

	h := ingest.New(idx, blobs, mocks.NewMockExtractor(ctrl), mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

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

	h := ingest.New(idx, mocks.NewMockStore(ctrl), mocks.NewMockExtractor(ctrl), mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

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

	h := ingest.New(idx, mocks.NewMockStore(ctrl), mocks.NewMockExtractor(ctrl), mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl))

	// Act
	err := h.Handle(context.Background(), s3Event("uploads/upl-1"))

	// Assert
	assert.Error(t, err)
}
