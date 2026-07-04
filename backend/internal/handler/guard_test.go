package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazemisoroush/vault/backend/internal/config"
	"github.com/kazemisoroush/vault/backend/internal/handler"
)

func TestGuardDisabledReturnsRoutesUngated(t *testing.T) {
	// Arrange
	reached := false
	routes := http.NewServeMux()
	routes.HandleFunc("/files", func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})

	// Act
	got, err := handler.Guard(context.Background(), config.Config{AuthDisabled: true}, routes)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	got.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files", nil))

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, reached, "disabled auth should pass the request straight through")
}

func TestGuardNotConfiguredFailsClosed(t *testing.T) {
	// Arrange
	cfg := config.Config{}

	// Act
	got, err := handler.Guard(context.Background(), cfg, http.NewServeMux())

	// Assert
	assert.Error(t, err)
	assert.Nil(t, got)
}
