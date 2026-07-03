package handler_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/handler"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// okNext is a downstream handler that records it was reached.
func okNext(reached *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireAuth(t *testing.T) {
	// Arrange
	tests := []struct {
		name       string
		path       string
		authHeader string
		verifyErr  error
		expectCall bool
		wantStatus int
		wantNext   bool
	}{
		{name: "health is public", path: "/health", expectCall: false, wantStatus: http.StatusOK, wantNext: true},
		{name: "missing header is rejected", path: "/files", expectCall: false, wantStatus: http.StatusUnauthorized, wantNext: false},
		{name: "valid token passes", path: "/files", authHeader: "Bearer good", verifyErr: nil, expectCall: true, wantStatus: http.StatusOK, wantNext: true},
		{name: "invalid token is rejected", path: "/files", authHeader: "Bearer bad", verifyErr: errors.New("nope"), expectCall: true, wantStatus: http.StatusUnauthorized, wantNext: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			verifier := mocks.NewMockTokenVerifier(ctrl)
			if tc.expectCall {
				verifier.EXPECT().Verify(gomock.Any()).Return(tc.verifyErr)
			}
			reached := false
			mw := handler.RequireAuth(okNext(&reached), verifier)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()

			// Act
			mw.ServeHTTP(rec, req)

			// Assert
			assert.Equal(t, tc.wantStatus, rec.Code)
			assert.Equal(t, tc.wantNext, reached)
		})
	}
}
