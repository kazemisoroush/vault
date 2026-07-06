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

		m.emitter.Emit(telemetry.Namespace,
			map[string]string{"Route": route(r), "Method": methodLabel(r.Method), "Status": statusClass(recorder.status)},
			telemetry.Metric{Name: "RequestCount", Value: 1, Unit: telemetry.Count},
			telemetry.Metric{Name: "RequestDurationMs", Value: float64(m.now().Sub(start).Milliseconds()), Unit: telemetry.Milliseconds},
		)
	})
}

// knownMethods bounds the Method dimension so an arbitrary request method cannot inflate cardinality.
var knownMethods = map[string]bool{
	http.MethodGet: true, http.MethodHead: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true, http.MethodOptions: true,
	http.MethodConnect: true, http.MethodTrace: true,
}

// route returns the matched route pattern, or "unmatched" so an arbitrary path cannot inflate cardinality.
func route(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	return "unmatched"
}

// methodLabel returns the request method when it is a known HTTP method, else "other".
func methodLabel(method string) string {
	if knownMethods[method] {
		return method
	}
	return "other"
}

// statusClass buckets a status into its class (2xx, 4xx, ...) to keep metric cardinality low.
func statusClass(status int) string {
	return strconv.Itoa(status/100) + "xx"
}
