// Package llm wraps the Anthropic model served by Amazon Bedrock.
package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/aws/aws-sdk-go-v2/config"
)

// Model is an Anthropic model backed by Amazon Bedrock.
type Model struct {
	client anthropic.Client
	name   string
}

// NewModel builds a Model for a Bedrock region and model id.
func NewModel(region, name string) *Model {
	client := anthropic.NewClient(bedrock.WithLoadDefaultConfig(context.Background(), config.WithRegion(region)))
	return &Model{client: client, name: name}
}

// Complete sends the user content blocks to the model and returns its aggregated text reply.
func (m *Model) Complete(ctx context.Context, maxTokens int64, blocks ...anthropic.ContentBlockParamUnion) (string, error) {
	resp, err := m.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(m.name),
		MaxTokens: maxTokens,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)},
	})
	if err != nil {
		return "", fmt.Errorf("bedrock complete: %w", err)
	}

	var reply strings.Builder
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			reply.WriteString(text.Text)
		}
	}
	return reply.String(), nil
}
