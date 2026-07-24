package controller

import (
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

func fillController(t *testing.T) (*FillController, *agentmock.MockAnswerer, *mocks.MockStore) {
	t.Helper()
	ctrl := gomock.NewController(t)
	answerer := agentmock.NewMockAnswerer(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	return NewFillController(answerer, blobs), answerer, blobs
}

func TestFillAnswersEachFieldInOrderWithItsSource(t *testing.T) {
	// Arrange: two fields; the first is sourced, the second the vault cannot back.
	c, answerer, blobs := fillController(t)
	passport := domain.File{ID: "p", Name: "passport", Key: "files/p"}
	answerer.EXPECT().Answer(gomock.Any(), gomock.Any(), "passport number").
		Return(agent.Result{Text: "RA3495037", Files: []domain.File{passport}}, nil)
	answerer.EXPECT().Answer(gomock.Any(), gomock.Any(), "visa subclass").
		Return(agent.Result{Text: "no idea", Files: nil}, nil) // no cited file -> not found
	blobs.EXPECT().PresignGet(gomock.Any(), "files/p", gomock.Any()).Return("https://get/p", nil)
	req := httptest.NewRequest(http.MethodPost, "/fill",
		strings.NewReader(`{"fields":["passport number","visa subclass"]}`))
	rec := httptest.NewRecorder()

	// Act
	c.Fill(rec, req)

	// Assert: answers stay in request order; the found one carries value + source, the unsourced one does not.
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Less(t, strings.Index(body, `"field":"passport number"`), strings.Index(body, `"field":"visa subclass"`))
	assert.Contains(t, body, `"value":"RA3495037"`)
	assert.Contains(t, body, `"downloadUrl":"https://get/p"`)
	assert.Contains(t, body, `"found":true`)
	assert.Contains(t, body, `"found":false`)
}

func TestFillTreatsAnAgentErrorAsNotFound(t *testing.T) {
	// Arrange: the agent errors on the one field. One field must never sink the whole form.
	c, answerer, _ := fillController(t)
	answerer.EXPECT().Answer(gomock.Any(), gomock.Any(), "medicare number").
		Return(agent.Result{}, assert.AnError)
	req := httptest.NewRequest(http.MethodPost, "/fill", strings.NewReader(`{"fields":["medicare number"]}`))
	rec := httptest.NewRecorder()

	// Act
	c.Fill(rec, req)

	// Assert: still 200, the field comes back not found rather than as an error.
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"found":false`)
}

func TestFillRejectsWhenEveryFieldIsBlank(t *testing.T) {
	// Arrange: blanks are trimmed away, leaving nothing to answer.
	c, _, _ := fillController(t)
	req := httptest.NewRequest(http.MethodPost, "/fill", strings.NewReader(`{"fields":["  ",""]}`))
	rec := httptest.NewRecorder()

	// Act
	c.Fill(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
