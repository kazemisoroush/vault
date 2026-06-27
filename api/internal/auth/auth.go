package auth

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

//go:generate go tool mockgen -source=auth.go -destination=auth_mock.go -package=auth

// TokenValidator verifies an identity token and returns the verified email.
type TokenValidator interface {
	Validate(ctx context.Context, token string) (string, error)
}

// Middleware enforces that requests carry a valid token belonging to the owner.
type Middleware struct {
	validator  TokenValidator
	ownerEmail string
}

// NewMiddleware creates an auth middleware for a single owner email.
func NewMiddleware(validator TokenValidator, ownerEmail string) *Middleware {
	return &Middleware{validator: validator, ownerEmail: ownerEmail}
}

// Wrap requires a valid owner token on every request before calling next.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeAuthError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		email, err := m.validator.Validate(r.Context(), token)
		if err != nil {
			log.Printf("auth: token validation failed: %v", err)
			writeAuthError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		if !strings.EqualFold(strings.TrimSpace(email), strings.TrimSpace(m.ownerEmail)) {
			writeAuthError(w, http.StatusForbidden, "forbidden")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
