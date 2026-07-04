package extract

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// instruction tells the model to return only a flat JSON metadata object.
const instruction = `You are extracting metadata from a personal file (often a receipt, ticket, or document image).
Return ONLY a flat JSON object mapping string keys to string values that a person would search by,
for example vendor, amount, date, place, person, event, or document type.
Use whatever keys the file actually carries. No nesting, no arrays, no commentary.`

// maxTokens caps the model reply to the size of a small flat metadata object.
const maxTokens = 1024

// ClaudeExtractor extracts metadata using Claude on Amazon Bedrock.
type ClaudeExtractor struct {
	model *llm.Model
}

// NewClaudeExtractor builds a ClaudeExtractor for a Bedrock region and model.
func NewClaudeExtractor(_ context.Context, region, model string, recorder llm.Recorder) (*ClaudeExtractor, error) {
	return &ClaudeExtractor{model: llm.NewModel(region, model, "extract", recorder)}, nil
}

// Extract sends the file to the model and returns its flat metadata map.
func (e *ClaudeExtractor) Extract(ctx context.Context, content []byte, contentType string) (map[string]string, error) {
	prompt := fmt.Sprintf("%s\n\n[file: %s, %d bytes]", instruction, contentType, len(content))
	reply, err := e.model.Complete(ctx, prompt, maxTokens, fileBlock(content, contentType), anthropic.NewTextBlock(instruction))
	if err != nil {
		return nil, fmt.Errorf("bedrock extract: %w", err)
	}

	meta, err := parseMeta(reply)
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
