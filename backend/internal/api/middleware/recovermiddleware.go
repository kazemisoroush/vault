package middleware

import (
	"log"
	"net/http"
)

// RecoverMiddleware turns a panic in a downstream handler into a 500 JSON response.
type RecoverMiddleware struct{}

// NewRecoverMiddleware builds a RecoverMiddleware.
func NewRecoverMiddleware() *RecoverMiddleware {
	return &RecoverMiddleware{}
}

// Wrap recovers any panic in the next handler and returns a 500.
func (m *RecoverMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("recovered panic serving %s %s: %v", r.Method, r.URL.Path, rec)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
