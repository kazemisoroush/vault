// Package llm wraps the Anthropic model served by Amazon Bedrock and records each call.
package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/aws/aws-sdk-go-v2/config"
)

// Model is an Anthropic model backed by Amazon Bedrock. Every Complete is recorded.
type Model struct {
	client   anthropic.Client
	name     string
	op       string
	recorder Recorder
}

// NewModel builds a Model for a Bedrock region and model id, tagging its calls with op.
func NewModel(region, name, op string, recorder Recorder) *Model {
	client := anthropic.NewClient(bedrock.WithLoadDefaultConfig(context.Background(), config.WithRegion(region)))
	return &Model{client: client, name: name, op: op, recorder: recorder}
}

// Complete sends the user content blocks to the model, records the call, and returns the reply.
// prompt is the human-readable prompt kept for the trace; it need not include raw file bytes.
func (m *Model) Complete(ctx context.Context, prompt string, maxTokens int64, blocks ...anthropic.ContentBlockParamUnion) (string, error) {
	start := time.Now()
	resp, err := m.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(m.name),
		MaxTokens: maxTokens,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)},
	})
	call := Call{
		Op:        m.op,
		Model:     m.name,
		Prompt:    prompt,
		LatencyMS: time.Since(start).Milliseconds(),
		CreatedAt: start.UTC(),
	}
	if err != nil {
		call.Error = err.Error()
		m.recorder.Record(ctx, call)
		return "", fmt.Errorf("bedrock complete: %w", err)
	}

	reply := collectText(resp.Content)
	call.OK = true
	call.Reply = reply
	call.InputTokens = resp.Usage.InputTokens
	call.OutputTokens = resp.Usage.OutputTokens
	m.recorder.Record(ctx, call)
	return reply, nil
}

// collectText concatenates the text blocks of a model response.
func collectText(blocks []anthropic.ContentBlockUnion) string {
	var reply strings.Builder
	for _, block := range blocks {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			reply.WriteString(text.Text)
		}
	}
	return reply.String()
}
