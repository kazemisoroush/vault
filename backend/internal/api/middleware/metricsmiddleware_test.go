package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazemisoroush/vault/backend/internal/api/middleware"
	"github.com/kazemisoroush/vault/backend/internal/telemetry"
)

// captureEmitter records the last Emit call so a test can assert on it.
type captureEmitter struct {
	dimensions map[string]string
	metrics    []telemetry.Metric
	calls      int
}

func (c *captureEmitter) Emit(_ string, dimensions map[string]string, metrics ...telemetry.Metric) {
	c.dimensions = dimensions
	c.metrics = metrics
	c.calls++
}

func TestMetricsMiddlewareRecordsMatchedRoute(t *testing.T) {
	// Arrange: a real mux so the request carries its matched pattern.
	emitter := &captureEmitter{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /files/{id}", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	handler := middleware.NewMetricsMiddleware(emitter).Wrap(mux)

	// Act
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/files/abc", nil))

	// Assert: the route is the pattern (not the concrete id), keeping cardinality low.
	require.Equal(t, 1, emitter.calls)
	assert.Equal(t, "GET /files/{id}", emitter.dimensions["Route"])
	assert.Equal(t, "GET", emitter.dimensions["Method"])
	assert.Equal(t, "2xx", emitter.dimensions["Status"])

	names := make([]string, 0, len(emitter.metrics))
	for _, metric := range emitter.metrics {
		names = append(names, metric.Name)
	}
	assert.ElementsMatch(t, []string{"RequestCount", "RequestDurationMs"}, names)
}

func TestMetricsMiddlewareRecordsErrorClass(t *testing.T) {
	// Arrange
	emitter := &captureEmitter{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /files", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	handler := middleware.NewMetricsMiddleware(emitter).Wrap(mux)

	// Act
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/files", nil))

	// Assert
	assert.Equal(t, "5xx", emitter.dimensions["Status"])
	assert.Equal(t, "POST /files", emitter.dimensions["Route"])
}

func TestMetricsMiddlewareFallsBackWhenNoRouteMatched(t *testing.T) {
	// Arrange: no mux, so the request has no matched pattern (as with an auth rejection).
	emitter := &captureEmitter{}
	handler := middleware.NewMetricsMiddleware(emitter).Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	// Act
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/files", nil))

	// Assert
	assert.Equal(t, "GET /files", emitter.dimensions["Route"])
	assert.Equal(t, "4xx", emitter.dimensions["Status"])
}
