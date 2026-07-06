package middleware

import (
	"fmt"
	"net/http"
)

// statusRecorder wraps a ResponseWriter to remember the status code the handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// WriteHeader records the first status written before passing it through.
func (r *statusRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status = status
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(status)
}

// Write passes the body through, marking that an implicit 200 status now stands.
func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	written, err := r.ResponseWriter.Write(b)
	if err != nil {
		return written, fmt.Errorf("write response: %w", err)
	}
	return written, nil
}
