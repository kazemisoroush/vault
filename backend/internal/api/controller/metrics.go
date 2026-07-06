package controller

import (
	"encoding/json"
	"net/http"

	"github.com/kazemisoroush/vault/backend/internal/telemetry"
)

// maxTimeToFileMs caps a reported time-to-file so a bad client cannot skew the metric.
const maxTimeToFileMs = 600_000

// MetricsController turns a client-reported measurement into a telemetry metric.
type MetricsController struct {
	emitter telemetry.Emitter
}

// NewMetricsController builds a MetricsController over a telemetry emitter.
func NewMetricsController(emitter telemetry.Emitter) *MetricsController {
	return &MetricsController{emitter: emitter}
}

// timeToFileRequest is the body of a POST /metrics/time-to-file call.
type timeToFileRequest struct {
	Ms float64 `json:"ms"`
}

// TimeToFile records the client-measured milliseconds from asking to opening a file.
func (c *MetricsController) TimeToFile(w http.ResponseWriter, r *http.Request) {
	var req timeToFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Ms < 0 || req.Ms > maxTimeToFileMs {
		writeError(w, http.StatusBadRequest, "ms must be between 0 and 600000")
		return
	}

	c.emitter.Emit(telemetry.Namespace, map[string]string{"Source": "web"},
		telemetry.Metric{Name: "TimeToFileMs", Value: req.Ms, Unit: telemetry.Milliseconds})
	w.WriteHeader(http.StatusNoContent)
}
