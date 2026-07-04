package controller

import "net/http"

// HealthController reports service liveness.
type HealthController struct{}

// NewHealthController builds a health controller.
func NewHealthController() *HealthController {
	return &HealthController{}
}

func (c *HealthController) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
