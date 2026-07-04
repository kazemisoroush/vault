package llm

import "time"

// Call is one recorded LLM invocation, used for telemetry and the trace view.
type Call struct {
	Op           string    `json:"op"`
	Model        string    `json:"model"`
	Prompt       string    `json:"prompt"`
	Reply        string    `json:"reply"`
	InputTokens  int64     `json:"inputTokens"`
	OutputTokens int64     `json:"outputTokens"`
	LatencyMS    int64     `json:"latencyMs"`
	OK           bool      `json:"ok"`
	Error        string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}
