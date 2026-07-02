package handler

import (
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

func newTestHandler(t *testing.T) (*Handler, *mocks.MockIndex, *mocks.MockStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	h := New(idx, blobs)
	h.now = func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) }
	h.newID = func() string { return "test-id" }

	return h, idx, blobs
}

func TestCreateFile(t *testing.T) {
	// Arrange
	h, idx, blobs := newTestHandler(t)
	blobs.EXPECT().PresignPut(gomock.Any(), "files/test-id", "image/jpeg", presignExpiry).Return("https://upload", nil)
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).Return(nil)
	body := `{"name":"petrol-receipt.jpg","contentType":"image/jpeg","size":123}`
	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(body))
	rec := httptest.NewRecorder()

	// Act
	h.Routes().ServeHTTP(rec, req)

	// Assert
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), `"uploadUrl":"https://upload"`)
	assert.Contains(t, rec.Body.String(), `"id":"test-id"`)
}

func TestCreateFileRejectsMissingFields(t *testing.T) {
	// Arrange
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(`{"name":"x"}`))
	rec := httptest.NewRecorder()

	// Act
	h.Routes().ServeHTTP(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetFileNotFound(t *testing.T) {
	// Arrange
	h, idx, _ := newTestHandler(t)
	idx.EXPECT().Get(gomock.Any(), "missing").Return(domain.File{}, index.ErrNotFound)
	req := httptest.NewRequest(http.MethodGet, "/files/missing", nil)
	rec := httptest.NewRecorder()

	// Act
	h.Routes().ServeHTTP(rec, req)

	// Assert
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListFilesRejectsBadLimit(t *testing.T) {
	// Arrange
	h, _, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/files?limit=-5", nil)
	rec := httptest.NewRecorder()

	// Act
	h.Routes().ServeHTTP(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteFileRemovesRecordThenBlob(t *testing.T) {
	// Arrange
	h, idx, blobs := newTestHandler(t)
	file := domain.File{ID: "test-id", Key: "files/test-id"}
	idx.EXPECT().Get(gomock.Any(), "test-id").Return(file, nil)
	blobs.EXPECT().Delete(gomock.Any(), "files/test-id").Return(nil)
	idx.EXPECT().Delete(gomock.Any(), "test-id").Return(nil)
	req := httptest.NewRequest(http.MethodDelete, "/files/test-id", nil)
	rec := httptest.NewRecorder()

	// Act
	h.Routes().ServeHTTP(rec, req)

	// Assert
	require.Equal(t, http.StatusNoContent, rec.Code)
}
