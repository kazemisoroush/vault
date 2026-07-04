package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/llm"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

func TestCallsReturnsRecent(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	lister := mocks.NewMockCallLister(ctrl)
	lister.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]llm.Call{{Op: "retrieve", Model: "haiku", OK: true, CreatedAt: time.Unix(1, 0)}}, nil)
	c := NewCallsController(lister)
	req := httptest.NewRequest(http.MethodGet, "/calls", nil)
	rec := httptest.NewRecorder()

	// Act
	c.Calls(rec, req)

	// Assert
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"op":"retrieve"`)
}

func TestCallsRejectsBadLimit(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	lister := mocks.NewMockCallLister(ctrl)
	c := NewCallsController(lister)
	req := httptest.NewRequest(http.MethodGet, "/calls?limit=0", nil)
	rec := httptest.NewRecorder()

	// Act
	c.Calls(rec, req)

	// Assert
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
