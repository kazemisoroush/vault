package extract

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/bedrock"
)

// instruction tells the model to return only a flat JSON metadata object.
const instruction = `You are extracting metadata from a personal file (often a receipt, ticket, or document image).
Return ONLY a flat JSON object mapping string keys to string values that a person would search by,
for example vendor, amount, date, place, person, event, or document type.
Use whatever keys the file actually carries. No nesting, no arrays, no commentary.`

// maxTokens caps the model reply; a flat metadata object is small.
const maxTokens = 1024

// ClaudeExtractor extracts metadata using Claude on Amazon Bedrock.
type ClaudeExtractor struct {
	client *bedrock.MantleClient
	model  string
}

// NewClaudeExtractor builds a ClaudeExtractor for a Bedrock region and model.
func NewClaudeExtractor(ctx context.Context, region, model string) (*ClaudeExtractor, error) {
	client, err := bedrock.NewMantleClient(ctx, bedrock.MantleClientConfig{AWSRegion: region})
	if err != nil {
		return nil, fmt.Errorf("build bedrock client: %w", err)
	}
	return &ClaudeExtractor{client: client, model: model}, nil
}

// Extract sends the file to the model and returns its flat metadata map.
func (e *ClaudeExtractor) Extract(ctx context.Context, content []byte, contentType string) (map[string]string, error) {
	message := anthropic.NewUserMessage(fileBlock(content, contentType), anthropic.NewTextBlock(instruction))

	resp, err := e.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(e.model),
		MaxTokens: maxTokens,
		Messages:  []anthropic.MessageParam{message},
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock extract: %w", err)
	}

	var reply strings.Builder
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			reply.WriteString(text.Text)
		}
	}

	meta, err := parseMeta(reply.String())
	if err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}
	return meta, nil
}

// fileBlock wraps the bytes as the content block that fits the content type.
func fileBlock(content []byte, contentType string) anthropic.ContentBlockParamUnion {
	encoded := base64.StdEncoding.EncodeToString(content)
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return anthropic.NewImageBlockBase64(contentType, encoded)
	case contentType == "application/pdf":
		return anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{Data: encoded})
	default:
		return anthropic.NewTextBlock(string(content))
	}
}
