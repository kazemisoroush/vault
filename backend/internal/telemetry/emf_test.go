package telemetry

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEMFEmitterWritesRecord(t *testing.T) {
	// Arrange
	var buf bytes.Buffer
	emitter := NewEMFEmitter(&buf)
	emitter.now = func() time.Time { return time.UnixMilli(1_700_000_000_000) }

	// Act
	emitter.Emit("Vault", map[string]string{"Route": "POST /files", "Method": "POST"},
		Metric{Name: "RequestCount", Value: 1, Unit: "Count"},
		Metric{Name: "RequestDurationMs", Value: 12, Unit: "Milliseconds"})

	// Assert: a single EMF record CloudWatch can extract.
	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
	assert.Equal(t, "POST /files", record["Route"])
	assert.Equal(t, "POST", record["Method"])
	assert.Equal(t, float64(1), record["RequestCount"])
	assert.Equal(t, float64(12), record["RequestDurationMs"])

	aws := record["_aws"].(map[string]any)
	assert.Equal(t, float64(1_700_000_000_000), aws["Timestamp"])
	metrics := aws["CloudWatchMetrics"].([]any)[0].(map[string]any)
	assert.Equal(t, "Vault", metrics["Namespace"])
	assert.ElementsMatch(t, []any{"Method", "Route"}, metrics["Dimensions"].([]any)[0].([]any))
	assert.Len(t, metrics["Metrics"].([]any), 2)
}

func TestEMFEmitterWithoutMetricsWritesNothing(t *testing.T) {
	// Arrange
	var buf bytes.Buffer

	// Act
	NewEMFEmitter(&buf).Emit("Vault", map[string]string{"Route": "GET /files"})

	// Assert
	assert.Empty(t, buf.Bytes())
}
