// Package llm wraps the Anthropic model served by Amazon Bedrock and records each call.
package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/aws/aws-sdk-go-v2/config"
)

// messenger is the slice of the Anthropic client the Model uses, kept small so tests can fake it.
type messenger interface {
	New(ctx context.Context, body anthropic.MessageNewParams, opts ...option.RequestOption) (*anthropic.Message, error)
}

// Model is an Anthropic model backed by Amazon Bedrock. Every call to the model is recorded.
type Model struct {
	client   messenger
	name     string
	op       string
	recorder Recorder
}

// NewModel builds a Model for a Bedrock region and model id, tagging its calls with op.
func NewModel(region, name, op string, recorder Recorder) *Model {
	client := anthropic.NewClient(bedrock.WithLoadDefaultConfig(context.Background(), config.WithRegion(region)))
	return newModel(&client.Messages, name, op, recorder)
}

// newModel builds a Model over a given messenger. NewModel and the tests share it.
func newModel(client messenger, name, op string, recorder Recorder) *Model {
	return &Model{client: client, name: name, op: op, recorder: recorder}
}

// send makes one model call and records it, whether it fails or returns. label is the
// human-readable prompt kept for the trace. It is the single place a model call is recorded.
func (m *Model) send(ctx context.Context, label string, params anthropic.MessageNewParams) (*anthropic.Message, error) {
	start := time.Now()
	resp, err := m.client.New(ctx, params)
	call := Call{
		Op:        m.op,
		Model:     m.name,
		Prompt:    label,
		LatencyMS: time.Since(start).Milliseconds(),
		CreatedAt: start.UTC(),
	}
	if err != nil {
		call.Error = err.Error()
		m.recorder.Record(ctx, call)
		return nil, fmt.Errorf("bedrock complete: %w", err)
	}

	call.OK = true
	call.Reply = collectText(resp.Content)
	call.InputTokens = resp.Usage.InputTokens
	call.OutputTokens = resp.Usage.OutputTokens
	m.recorder.Record(ctx, call)
	return resp, nil
}

// collectText concatenates the text blocks of a model response, read from the flattened union
// fields so it does not depend on the raw JSON that AsAny needs.
func collectText(blocks []anthropic.ContentBlockUnion) string {
	var reply strings.Builder
	for _, block := range blocks {
		if block.Type == "text" {
			reply.WriteString(block.Text)
		}
	}
	return reply.String()
}
