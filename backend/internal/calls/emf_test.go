package calls

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

func TestEMFLine(t *testing.T) {
	tests := []struct {
		name       string
		ok         bool
		wantErrors float64
	}{
		{name: "success", ok: true, wantErrors: 0},
		{name: "failure", ok: false, wantErrors: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			call := llm.Call{Op: "retrieve", Model: "haiku", LatencyMS: 1200, InputTokens: 500, OutputTokens: 20, OK: tc.ok, CreatedAt: time.Unix(1, 0)}

			// Act
			var doc map[string]any
			require.NoError(t, json.Unmarshal([]byte(emfLine(call)), &doc))

			// Assert
			require.Equal(t, "retrieve", doc["Op"])
			require.EqualValues(t, 1200, doc["LatencyMs"])
			require.EqualValues(t, tc.wantErrors, doc["Errors"])
			require.Contains(t, doc, "_aws")
		})
	}
}
