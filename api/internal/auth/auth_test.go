package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func newRequest(authHeader string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	return req
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestWrap_MissingToken(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	validator := NewMockTokenValidator(ctrl)
	mw := NewMiddleware(validator, "owner@example.com")
	rec := httptest.NewRecorder()

	// Act
	mw.Wrap(okHandler()).ServeHTTP(rec, newRequest(""))

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWrap_InvalidToken(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	validator := NewMockTokenValidator(ctrl)
	validator.EXPECT().Validate(gomock.Any(), "bad").Return("", errors.New("invalid"))
	mw := NewMiddleware(validator, "owner@example.com")
	rec := httptest.NewRecorder()

	// Act
	mw.Wrap(okHandler()).ServeHTTP(rec, newRequest("Bearer bad"))

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWrap_WrongOwner(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	validator := NewMockTokenValidator(ctrl)
	validator.EXPECT().Validate(gomock.Any(), "tok").Return("intruder@example.com", nil)
	mw := NewMiddleware(validator, "owner@example.com")
	rec := httptest.NewRecorder()

	// Act
	mw.Wrap(okHandler()).ServeHTTP(rec, newRequest("Bearer tok"))

	// Assert
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestWrap_OwnerAllowed(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	validator := NewMockTokenValidator(ctrl)
	validator.EXPECT().Validate(gomock.Any(), "tok").Return("Owner@Example.com", nil)
	mw := NewMiddleware(validator, "owner@example.com")
	rec := httptest.NewRecorder()

	// Act
	mw.Wrap(okHandler()).ServeHTTP(rec, newRequest("Bearer tok"))

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
}
