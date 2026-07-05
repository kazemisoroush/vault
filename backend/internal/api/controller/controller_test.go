package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

func mockDeps(t *testing.T) (*mocks.MockIndex, *mocks.MockStore, *mocks.MockEmbedder, *mocks.MockVectorStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	return mocks.NewMockIndex(ctrl), mocks.NewMockStore(ctrl), mocks.NewMockEmbedder(ctrl), mocks.NewMockVectorStore(ctrl)
}

func TestDropCreatesPendingRecord(t *testing.T) {
	// Arrange
	idx, blobs, embedder, store := mockDeps(t)
	c := NewFileController(idx, blobs, embedder, store)
	c.now = func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) }
	c.newID = func() string { return "test-id" }
	blobs.EXPECT().PresignPut(gomock.Any(), "files/test-id", "image/jpeg", presignExpiry).Return("https://upload", nil)
	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})
	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(`{"name":"petrol-receipt.jpg","contentType":"image/jpeg","size":123}`))
	rec := httptest.NewRecorder()

	// Act
	c.Drop(rec, req)

	// Assert
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, domain.StatusPending, saved.Status)
	assert.Equal(t, "files/test-id", saved.Key)
	assert.Contains(t, rec.Body.String(), `"uploadUrl":"https://upload"`)
	assert.Contains(t, rec.Body.String(), `"id":"test-id"`)
}

func TestDropRejectsMissingFields(t *testing.T) {
	// Arrange
	idx, blobs, embedder, store := mockDeps(t)
	c := NewFileController(idx, blobs, embedder, store)
	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(`{"name":"x"}`))
	rec := httptest.NewRecorder()

	// Act
	c.Drop(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetNotFound(t *testing.T) {
	// Arrange
	idx, blobs, embedder, store := mockDeps(t)
	idx.EXPECT().Get(gomock.Any(), "missing").Return(domain.File{}, index.ErrNotFound)
	c := NewFileController(idx, blobs, embedder, store)
	req := httptest.NewRequest(http.MethodGet, "/files/missing", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()

	// Act
	c.Get(rec, req)

	// Assert
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListRejectsBadLimit(t *testing.T) {
	// Arrange
	idx, blobs, embedder, store := mockDeps(t)
	c := NewFileController(idx, blobs, embedder, store)
	req := httptest.NewRequest(http.MethodGet, "/files?limit=-5", nil)
	rec := httptest.NewRecorder()

	// Act
	c.List(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteRemovesRecordThenBlob(t *testing.T) {
	// Arrange
	idx, blobs, embedder, store := mockDeps(t)
	file := domain.File{ID: "test-id", Key: "files/test-id"}
	idx.EXPECT().Get(gomock.Any(), "test-id").Return(file, nil)
	idx.EXPECT().Delete(gomock.Any(), "test-id").Return(nil)
	blobs.EXPECT().Delete(gomock.Any(), "files/test-id").Return(nil)
	store.EXPECT().Delete(gomock.Any(), "test-id").Return(nil)
	c := NewFileController(idx, blobs, embedder, store)
	req := httptest.NewRequest(http.MethodDelete, "/files/test-id", nil)
	req.SetPathValue("id", "test-id")
	rec := httptest.NewRecorder()

	// Act
	c.Delete(rec, req)

	// Assert
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestUpdateRenameReembeds(t *testing.T) {
	// Arrange
	idx, blobs, embedder, store := mockDeps(t)
	file := domain.File{ID: "id1", Key: "files/id1", Name: "old.txt", Meta: map[string]string{"vendor": "Shell"}}
	idx.EXPECT().Get(gomock.Any(), "id1").Return(file, nil)
	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})
	embedder.EXPECT().Embed(gomock.Any(), gomock.Any()).Return([]float32{0.1, 0.2}, nil)
	store.EXPECT().Put(gomock.Any(), "id1", []float32{0.1, 0.2}).Return(nil)
	c := NewFileController(idx, blobs, embedder, store)
	req := httptest.NewRequest(http.MethodPatch, "/files/id1", strings.NewReader(`{"name":"new.txt"}`))
	req.SetPathValue("id", "id1")
	rec := httptest.NewRecorder()

	// Act
	c.Update(rec, req)

	// Assert: the record is renamed and its vector was refreshed.
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "new.txt", saved.Name)
}
