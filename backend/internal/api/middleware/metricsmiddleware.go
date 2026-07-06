package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/kazemisoroush/vault/backend/internal/telemetry"
)

// MetricsMiddleware records RED metrics (request count, errors via status, duration) for every route.
type MetricsMiddleware struct {
	emitter telemetry.Emitter
	now     func() time.Time
}

// NewMetricsMiddleware builds a MetricsMiddleware over a telemetry emitter.
func NewMetricsMiddleware(emitter telemetry.Emitter) *MetricsMiddleware {
	return &MetricsMiddleware{emitter: emitter, now: time.Now}
}

// Wrap times the next handler and emits per-request count and duration by route, method, and status.
func (m *MetricsMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := m.now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		m.emitter.Emit("Vault",
			map[string]string{"Route": route(r), "Method": r.Method, "Status": statusClass(recorder.status)},
			telemetry.Metric{Name: "RequestCount", Value: 1, Unit: "Count"},
			telemetry.Metric{Name: "RequestDurationMs", Value: float64(m.now().Sub(start).Milliseconds()), Unit: "Milliseconds"},
		)
	})
}

// route returns the matched route pattern, or the method and path when no route matched (for example a 401).
func route(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return r.Method + " " + r.URL.Path
}

// statusClass buckets a status into its class (2xx, 4xx, ...) to keep metric cardinality low.
func statusClass(status int) string {
	return strconv.Itoa(status/100) + "xx"
}
