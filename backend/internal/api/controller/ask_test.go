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

	"github.com/kazemisoroush/vault/backend/internal/agent"
	agentmock "github.com/kazemisoroush/vault/backend/internal/agent/mock"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

func askController(t *testing.T) (*AskController, *agentmock.MockAnswerer, *mocks.MockStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	answerer := agentmock.NewMockAnswerer(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	return NewAskController(answerer, blobs), answerer, blobs
}

func TestAskReturnsTheAnswerAndItsFilesAsLinks(t *testing.T) {
	// Arrange: the agent answers with two files it used, in order.
	c, answerer, blobs := askController(t)
	files := []domain.File{
		{ID: "b", Name: "two", Key: "files/b"},
		{ID: "a", Name: "one", Key: "files/a"},
	}
	answerer.EXPECT().Answer(gomock.Any(), gomock.Any(), "petrol receipts").
		Return(agent.Result{Text: "your passport number is RA3495037", Files: files}, nil)
	blobs.EXPECT().PresignGet(gomock.Any(), "files/b", gomock.Any()).Return("https://get/b", nil)
	blobs.EXPECT().PresignGet(gomock.Any(), "files/a", gomock.Any()).Return("https://get/a", nil)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"petrol receipts"}`))
	rec := httptest.NewRecorder()

	// Act
	c.Ask(rec, req)

	// Assert: answer plus each used file, in order, with a download link.
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Less(t, strings.Index(body, `"id":"b"`), strings.Index(body, `"id":"a"`))
	assert.Contains(t, body, `"downloadUrl":"https://get/b"`)
	assert.Contains(t, body, `"answer":"your passport number is RA3495037"`)
}

func TestAskRejectsEmptyQuery(t *testing.T) {
	// Arrange
	c, _, _ := askController(t)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"   "}`))
	rec := httptest.NewRecorder()

	// Act
	c.Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAskReturnsAnAnswerWithNoFiles(t *testing.T) {
	// Arrange: a plain answer that cited no files.
	c, answerer, _ := askController(t)
	answerer.EXPECT().Answer(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(agent.Result{Text: "nothing matched", Files: nil}, nil)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"anything"}`))
	rec := httptest.NewRecorder()

	// Act
	c.Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, strings.Count(rec.Body.String(), `"downloadUrl"`))
	assert.Contains(t, rec.Body.String(), `"answer":"nothing matched"`)
}

func TestAskReturnsErrorWhenTheAgentFails(t *testing.T) {
	// Arrange
	c, answerer, _ := askController(t)
	answerer.EXPECT().Answer(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(agent.Result{}, context.DeadlineExceeded)
	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"anything"}`))
	rec := httptest.NewRecorder()

	// Act
	c.Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}
