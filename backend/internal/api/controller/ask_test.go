package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

func askDeps(t *testing.T) (*mocks.MockIndex, *mocks.MockStore, *mocks.MockRetriever) {
	t.Helper()
	ctrl := gomock.NewController(t)
	return mocks.NewMockIndex(ctrl), mocks.NewMockStore(ctrl), mocks.NewMockRetriever(ctrl)
}

func TestAskReturnsMatchesInOrder(t *testing.T) {
	// Arrange
	idx, blobs, retriever := askDeps(t)
	files := []domain.File{
		{ID: "a", Name: "one", Key: "files/a"},
		{ID: "b", Name: "two", Key: "files/b"},
	}
	idx.EXPECT().List(gomock.Any(), gomock.Any(), "").Return(files, "", nil)
	retriever.EXPECT().Match(gomock.Any(), "petrol receipts", files).Return([]string{"b", "a"}, nil)
	blobs.EXPECT().PresignGet(gomock.Any(), "files/b", gomock.Any()).Return("https://get/b", nil)
	blobs.EXPECT().PresignGet(gomock.Any(), "files/a", gomock.Any()).Return("https://get/a", nil)
	c := NewAskController(idx, blobs, retriever)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"petrol receipts"}`))
	rec := httptest.NewRecorder()

	// Act
	c.Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Less(t, strings.Index(body, `"id":"b"`), strings.Index(body, `"id":"a"`))
	assert.Contains(t, body, `"downloadUrl":"https://get/b"`)
}

func TestAskRejectsEmptyQuery(t *testing.T) {
	// Arrange
	idx, blobs, retriever := askDeps(t)
	c := NewAskController(idx, blobs, retriever)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"   "}`))
	rec := httptest.NewRecorder()

	// Act
	c.Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAskSkipsUnknownIDs(t *testing.T) {
	// Arrange
	idx, blobs, retriever := askDeps(t)
	files := []domain.File{{ID: "a", Name: "one", Key: "files/a"}}
	idx.EXPECT().List(gomock.Any(), gomock.Any(), "").Return(files, "", nil)
	retriever.EXPECT().Match(gomock.Any(), gomock.Any(), files).Return([]string{"ghost", "a"}, nil)
	blobs.EXPECT().PresignGet(gomock.Any(), "files/a", gomock.Any()).Return("https://get/a", nil)
	c := NewAskController(idx, blobs, retriever)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"anything"}`))
	rec := httptest.NewRecorder()

	// Act
	c.Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, strings.Count(rec.Body.String(), `"downloadUrl"`))
}

func TestAskReturnsErrorWhenRetrieverFails(t *testing.T) {
	// Arrange
	idx, blobs, retriever := askDeps(t)
	idx.EXPECT().List(gomock.Any(), gomock.Any(), "").Return([]domain.File{}, "", nil)
	retriever.EXPECT().Match(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, context.DeadlineExceeded)
	c := NewAskController(idx, blobs, retriever)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"anything"}`))
	rec := httptest.NewRecorder()

	// Act
	c.Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}
