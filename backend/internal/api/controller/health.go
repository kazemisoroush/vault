package controller

import "net/http"

// Health reports service liveness.
type Health struct{}

// NewHealth builds a Health controller.
func NewHealth() *Health {
	return &Health{}
}

func (c *Health) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
