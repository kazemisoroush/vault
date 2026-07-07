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
	"github.com/kazemisoroush/vault/backend/internal/retrieve"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

type askMocks struct {
	index     *mocks.MockIndex
	blobs     *mocks.MockStore
	embedder  *mocks.MockEmbedder
	vectors   *mocks.MockVectorStore
	retriever *mocks.MockRetriever
}

func askDeps(t *testing.T) askMocks {
	t.Helper()
	ctrl := gomock.NewController(t)
	return askMocks{
		index:     mocks.NewMockIndex(ctrl),
		blobs:     mocks.NewMockStore(ctrl),
		embedder:  mocks.NewMockEmbedder(ctrl),
		vectors:   mocks.NewMockVectorStore(ctrl),
		retriever: mocks.NewMockRetriever(ctrl),
	}
}

func (m askMocks) controller() *AskController {
	return NewAskController(m.index, m.blobs, m.embedder, m.vectors, m.retriever)
}

func TestAskReturnsMatchesInOrder(t *testing.T) {
	// Arrange
	m := askDeps(t)
	vec := []float32{0.1, 0.2}
	files := []domain.File{
		{ID: "a", Name: "one", Key: "files/a"},
		{ID: "b", Name: "two", Key: "files/b"},
	}
	m.embedder.EXPECT().Embed(gomock.Any(), "petrol receipts").Return(vec, nil)
	m.vectors.EXPECT().Query(gomock.Any(), gomock.Any(), vec, shortlistSize).Return([]string{"a", "b"}, nil)
	m.index.EXPECT().Get(gomock.Any(), "a").Return(files[0], nil)
	m.index.EXPECT().Get(gomock.Any(), "b").Return(files[1], nil)
	m.retriever.EXPECT().Match(gomock.Any(), "petrol receipts", files).Return(retrieve.Answer{Text: "your passport number is RA3495037", IDs: []string{"b", "a"}}, nil)
	m.blobs.EXPECT().PresignGet(gomock.Any(), "files/b", gomock.Any()).Return("https://get/b", nil)
	m.blobs.EXPECT().PresignGet(gomock.Any(), "files/a", gomock.Any()).Return("https://get/a", nil)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"petrol receipts"}`))
	rec := httptest.NewRecorder()

	// Act
	m.controller().Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Less(t, strings.Index(body, `"id":"b"`), strings.Index(body, `"id":"a"`))
	assert.Contains(t, body, `"downloadUrl":"https://get/b"`)
	assert.Contains(t, body, `"answer":"your passport number is RA3495037"`)
}

func TestAskRejectsEmptyQuery(t *testing.T) {
	// Arrange
	m := askDeps(t)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"   "}`))
	rec := httptest.NewRecorder()

	// Act
	m.controller().Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAskSkipsUnknownIDs(t *testing.T) {
	// Arrange
	m := askDeps(t)
	vec := []float32{0.1}
	files := []domain.File{{ID: "a", Name: "one", Key: "files/a"}}
	m.embedder.EXPECT().Embed(gomock.Any(), gomock.Any()).Return(vec, nil)
	m.vectors.EXPECT().Query(gomock.Any(), gomock.Any(), vec, shortlistSize).Return([]string{"a"}, nil)
	m.index.EXPECT().Get(gomock.Any(), "a").Return(files[0], nil)
	m.retriever.EXPECT().Match(gomock.Any(), gomock.Any(), files).Return(retrieve.Answer{IDs: []string{"ghost", "a"}}, nil)
	m.blobs.EXPECT().PresignGet(gomock.Any(), "files/a", gomock.Any()).Return("https://get/a", nil)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"anything"}`))
	rec := httptest.NewRecorder()

	// Act
	m.controller().Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, strings.Count(rec.Body.String(), `"downloadUrl"`))
}

func TestAskSkipsRecordsThatVanished(t *testing.T) {
	// Arrange: the vector store still has an id whose record was deleted.
	m := askDeps(t)
	vec := []float32{0.1}
	m.embedder.EXPECT().Embed(gomock.Any(), gomock.Any()).Return(vec, nil)
	m.vectors.EXPECT().Query(gomock.Any(), gomock.Any(), vec, shortlistSize).Return([]string{"ghost"}, nil)
	m.index.EXPECT().Get(gomock.Any(), "ghost").Return(domain.File{}, context.DeadlineExceeded)
	m.retriever.EXPECT().Match(gomock.Any(), gomock.Any(), []domain.File{}).Return(retrieve.Answer{IDs: []string{}}, nil)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"anything"}`))
	rec := httptest.NewRecorder()

	// Act
	m.controller().Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, strings.Count(rec.Body.String(), `"downloadUrl"`))
}

func TestAskReturnsErrorWhenEmbedFails(t *testing.T) {
	// Arrange
	m := askDeps(t)
	m.embedder.EXPECT().Embed(gomock.Any(), gomock.Any()).Return(nil, context.DeadlineExceeded)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"anything"}`))
	rec := httptest.NewRecorder()

	// Act
	m.controller().Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAskReturnsErrorWhenRetrieverFails(t *testing.T) {
	// Arrange
	m := askDeps(t)
	vec := []float32{0.1}
	m.embedder.EXPECT().Embed(gomock.Any(), gomock.Any()).Return(vec, nil)
	m.vectors.EXPECT().Query(gomock.Any(), gomock.Any(), vec, shortlistSize).Return([]string{}, nil)
	m.retriever.EXPECT().Match(gomock.Any(), gomock.Any(), gomock.Any()).Return(retrieve.Answer{}, context.DeadlineExceeded)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"anything"}`))
	rec := httptest.NewRecorder()

	// Act
	m.controller().Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}
