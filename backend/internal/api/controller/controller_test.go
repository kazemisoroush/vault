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

func mockDeps(t *testing.T) (*mocks.MockIndex, *mocks.MockStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	return mocks.NewMockIndex(ctrl), mocks.NewMockStore(ctrl)
}

func TestDropCreatesPendingRecord(t *testing.T) {
	// Arrange
	idx, blobs := mockDeps(t)
	c := NewFileController(idx, blobs)
	c.now = func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) }
	c.newID = func() string { return "test-id" }
	blobs.EXPECT().PresignPut(gomock.Any(), "uploads/test-id", "image/jpeg", presignExpiry).Return("https://upload", nil)
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
	assert.Equal(t, "uploads/test-id", saved.Key)
	assert.Contains(t, rec.Body.String(), `"uploadUrl":"https://upload"`)
	assert.Contains(t, rec.Body.String(), `"id":"test-id"`)
}

func TestDropRejectsMissingFields(t *testing.T) {
	// Arrange
	idx, blobs := mockDeps(t)
	c := NewFileController(idx, blobs)
	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(`{"name":"x"}`))
	rec := httptest.NewRecorder()

	// Act
	c.Drop(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetNotFound(t *testing.T) {
	// Arrange
	idx, blobs := mockDeps(t)
	idx.EXPECT().Get(gomock.Any(), "missing").Return(domain.File{}, index.ErrNotFound)
	c := NewFileController(idx, blobs)
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
	idx, blobs := mockDeps(t)
	c := NewFileController(idx, blobs)
	req := httptest.NewRequest(http.MethodGet, "/files?limit=-5", nil)
	rec := httptest.NewRecorder()

	// Act
	c.List(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteRemovesRecordThenBlobAndMetadata(t *testing.T) {
	// Arrange
	idx, blobs := mockDeps(t)
	file := domain.File{ID: "test-id", Key: "files/test-id"}
	idx.EXPECT().Get(gomock.Any(), "test-id").Return(file, nil)
	idx.EXPECT().Delete(gomock.Any(), "test-id").Return(nil)
	blobs.EXPECT().Delete(gomock.Any(), "files/test-id").Return(nil)
	// The Knowledge Base metadata sidecar is removed too, so the next sync drops the file.
	blobs.EXPECT().Delete(gomock.Any(), "files/test-id.metadata.json").Return(nil)
	c := NewFileController(idx, blobs)
	req := httptest.NewRequest(http.MethodDelete, "/files/test-id", nil)
	req.SetPathValue("id", "test-id")
	rec := httptest.NewRecorder()

	// Act
	c.Delete(rec, req)

	// Assert
	require.Equal(t, http.StatusNoContent, rec.Code)
}
