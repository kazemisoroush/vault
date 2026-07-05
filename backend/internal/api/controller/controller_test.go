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

func mockDeps(t *testing.T) (*mocks.MockIndex, *mocks.MockStore, *mocks.MockVectorStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	return mocks.NewMockIndex(ctrl), mocks.NewMockStore(ctrl), mocks.NewMockVectorStore(ctrl)
}

// dropHash is a valid SHA-256 hex content id for the drop tests.
var dropHash = strings.Repeat("a", 64)

func dropBody(hash string) string {
	return `{"name":"petrol.jpg","contentType":"image/jpeg","size":123,"hash":"` + hash + `"}`
}

func TestDropCreatesPendingRecordKeyedByHash(t *testing.T) {
	// Arrange
	idx, blobs, store := mockDeps(t)
	c := NewFileController(idx, blobs, store)
	c.now = func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) }
	idx.EXPECT().Get(gomock.Any(), dropHash).Return(domain.File{}, index.ErrNotFound)
	blobs.EXPECT().PresignPut(gomock.Any(), "files/"+dropHash, "image/jpeg", presignExpiry).Return("https://upload", nil)
	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})
	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(dropBody(dropHash)))
	rec := httptest.NewRecorder()

	// Act
	c.Drop(rec, req)

	// Assert: the id and key are the content hash.
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, domain.StatusPending, saved.Status)
	assert.Equal(t, "files/"+dropHash, saved.Key)
	assert.Contains(t, rec.Body.String(), `"uploadUrl":"https://upload"`)
	assert.Contains(t, rec.Body.String(), `"id":"`+dropHash+`"`)
}

func TestDropDeduplicatesReadyFile(t *testing.T) {
	// Arrange: the same bytes are already stored and ready, so this is a duplicate.
	idx, blobs, store := mockDeps(t)
	c := NewFileController(idx, blobs, store)
	idx.EXPECT().Get(gomock.Any(), dropHash).Return(domain.File{ID: dropHash, Status: domain.StatusReady}, nil)
	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(dropBody(dropHash)))
	rec := httptest.NewRecorder()

	// Act
	c.Drop(rec, req)

	// Assert: existing record returned, no upload URL, no new write (no PresignPut/Put expected).
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "uploadUrl")
	assert.Contains(t, rec.Body.String(), `"id":"`+dropHash+`"`)
}

func TestDropRetriesUnfinishedUpload(t *testing.T) {
	// Arrange: a prior drop registered but never finished uploading, so retry re-issues an upload.
	idx, blobs, store := mockDeps(t)
	c := NewFileController(idx, blobs, store)
	idx.EXPECT().Get(gomock.Any(), dropHash).Return(domain.File{ID: dropHash, Status: domain.StatusPending}, nil)
	blobs.EXPECT().PresignPut(gomock.Any(), "files/"+dropHash, "image/jpeg", presignExpiry).Return("https://upload", nil)
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).Return(nil)
	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(dropBody(dropHash)))
	rec := httptest.NewRecorder()

	// Act
	c.Drop(rec, req)

	// Assert
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), `"uploadUrl":"https://upload"`)
}

func TestDropRejectsMissingFields(t *testing.T) {
	// Arrange
	idx, blobs, store := mockDeps(t)
	c := NewFileController(idx, blobs, store)
	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(`{"name":"x","contentType":"image/jpeg"}`))
	rec := httptest.NewRecorder()

	// Act: no hash.
	c.Drop(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDropRejectsMalformedHash(t *testing.T) {
	// Arrange
	idx, blobs, store := mockDeps(t)
	c := NewFileController(idx, blobs, store)
	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(dropBody("../not-a-hash")))
	rec := httptest.NewRecorder()

	// Act
	c.Drop(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetNotFound(t *testing.T) {
	// Arrange
	idx, blobs, store := mockDeps(t)
	idx.EXPECT().Get(gomock.Any(), "missing").Return(domain.File{}, index.ErrNotFound)
	c := NewFileController(idx, blobs, store)
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
	idx, blobs, store := mockDeps(t)
	c := NewFileController(idx, blobs, store)
	req := httptest.NewRequest(http.MethodGet, "/files?limit=-5", nil)
	rec := httptest.NewRecorder()

	// Act
	c.List(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteRemovesRecordThenBlob(t *testing.T) {
	// Arrange
	idx, blobs, store := mockDeps(t)
	file := domain.File{ID: "test-id", Key: "files/test-id"}
	idx.EXPECT().Get(gomock.Any(), "test-id").Return(file, nil)
	idx.EXPECT().Delete(gomock.Any(), "test-id").Return(nil)
	blobs.EXPECT().Delete(gomock.Any(), "files/test-id").Return(nil)
	store.EXPECT().Delete(gomock.Any(), "test-id").Return(nil)
	c := NewFileController(idx, blobs, store)
	req := httptest.NewRequest(http.MethodDelete, "/files/test-id", nil)
	req.SetPathValue("id", "test-id")
	rec := httptest.NewRecorder()

	// Act
	c.Delete(rec, req)

	// Assert
	require.Equal(t, http.StatusNoContent, rec.Code)
}
