package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

const bearerPrefix = "Bearer "

// AuthMiddleware rejects requests without a valid bearer token, leaving /health open.
type AuthMiddleware struct {
	verifier TokenVerifier
}

// NewAuthMiddleware builds an AuthMiddleware over a token verifier.
func NewAuthMiddleware(verifier TokenVerifier) *AuthMiddleware {
	return &AuthMiddleware{verifier: verifier}
}

// Wrap gates the next handler behind bearer-token verification.
func (m *AuthMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		token, ok := bearerToken(r)
		if !ok {
			unauthorized(w, "missing bearer token")
			return
		}
		if err := m.verifier.Verify(token); err != nil {
			unauthorized(w, "invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// bearerToken extracts the token from an Authorization: Bearer header.
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if len(header) < len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(bearerPrefix):])
	return token, token != ""
}

// unauthorized writes a 401 JSON error.
func unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
