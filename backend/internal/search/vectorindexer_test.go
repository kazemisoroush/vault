package search

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

func TestIndexEmbedsAndStores(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	embedder := mocks.NewMockEmbedder(ctrl)
	store := mocks.NewMockVectorStore(ctrl)
	file := domain.File{ID: "id1", Name: "petrol.txt", Meta: map[string]string{"vendor": "Shell"}}
	embedder.EXPECT().Embed(gomock.Any(), file.SearchText()).Return([]float32{0.1, 0.2}, nil)
	store.EXPECT().Put(gomock.Any(), "id1", []float32{0.1, 0.2}).Return(nil)
	indexer := NewVectorIndexer(embedder, store)

	// Act + Assert
	require.NoError(t, indexer.Index(context.Background(), file))
}

func TestIndexReturnsEmbedError(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	embedder := mocks.NewMockEmbedder(ctrl)
	store := mocks.NewMockVectorStore(ctrl)
	embedder.EXPECT().Embed(gomock.Any(), gomock.Any()).Return(nil, errors.New("model down"))
	indexer := NewVectorIndexer(embedder, store)

	// Act + Assert
	assert.Error(t, indexer.Index(context.Background(), domain.File{ID: "id1"}))
}

func TestRemoveDeletesTheVector(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	embedder := mocks.NewMockEmbedder(ctrl)
	store := mocks.NewMockVectorStore(ctrl)
	store.EXPECT().Delete(gomock.Any(), "id1").Return(nil)
	indexer := NewVectorIndexer(embedder, store)

	// Act + Assert
	require.NoError(t, indexer.Remove(context.Background(), "id1"))
}
