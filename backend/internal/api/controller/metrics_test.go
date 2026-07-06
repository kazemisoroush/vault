package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/mocks"
	"github.com/kazemisoroush/vault/backend/internal/telemetry"
)

func TestTimeToFileEmitsMetric(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	emitter := mocks.NewMockEmitter(ctrl)
	emitter.EXPECT().Emit("Vault", map[string]string{"Source": "web"}, gomock.Any()).
		Do(func(_ string, _ map[string]string, metrics ...telemetry.Metric) {
			require.Len(t, metrics, 1)
			assert.Equal(t, "TimeToFileMs", metrics[0].Name)
			assert.Equal(t, float64(1500), metrics[0].Value)
		})
	controller := NewMetricsController(emitter)

	// Act
	req := httptest.NewRequest(http.MethodPost, "/metrics/time-to-file", strings.NewReader(`{"ms":1500}`))
	rec := httptest.NewRecorder()
	controller.TimeToFile(rec, req)

	// Assert
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTimeToFileRejectsOutOfRange(t *testing.T) {
	// Arrange: emitter must not be called for a bad value.
	ctrl := gomock.NewController(t)
	emitter := mocks.NewMockEmitter(ctrl)
	controller := NewMetricsController(emitter)

	// Act
	req := httptest.NewRequest(http.MethodPost, "/metrics/time-to-file", strings.NewReader(`{"ms":9999999}`))
	rec := httptest.NewRecorder()
	controller.TimeToFile(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
