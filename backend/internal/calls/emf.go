// Package calls records LLM invocations for the trace view and CloudWatch metrics.
package calls

import (
	"encoding/json"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// emfLine renders a CloudWatch Embedded Metric Format line for one call, so CloudWatch
// extracts latency, token, and error metrics from the logs with no PutMetricData call.
func emfLine(call llm.Call) string {
	errorCount := 0
	if !call.OK {
		errorCount = 1
	}
	doc := map[string]any{
		"_aws": map[string]any{
			"Timestamp": call.CreatedAt.UnixMilli(),
			"CloudWatchMetrics": []map[string]any{{
				"Namespace":  "Vault/LLM",
				"Dimensions": [][]string{{"Op"}},
				"Metrics": []map[string]any{
					{"Name": "LatencyMs", "Unit": "Milliseconds"},
					{"Name": "InputTokens", "Unit": "Count"},
					{"Name": "OutputTokens", "Unit": "Count"},
					{"Name": "Calls", "Unit": "Count"},
					{"Name": "Errors", "Unit": "Count"},
				},
			}},
		},
		"Op":           call.Op,
		"Model":        call.Model,
		"LatencyMs":    call.LatencyMS,
		"InputTokens":  call.InputTokens,
		"OutputTokens": call.OutputTokens,
		"Calls":        1,
		"Errors":       errorCount,
	}
	line, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	return string(line)
}
