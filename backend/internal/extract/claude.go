package extract

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

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
	reply, err := e.model.Converse(ctx, llm.Conversation{
		Prompt:    prompt,
		Content:   []llm.Part{fileBlock(content, contentType), llm.Text(instruction)},
		MaxTokens: maxTokens,
	})
	if err != nil {
		return nil, wrapExtractError(err)
	}

	// Merge the model's metadata over the file's own embedded metadata, treating a declined reply as none.
	result := embeddedMeta(content, contentType)
	maps.Copy(result, metaFromReply(reply))
	return result, nil
}

// wrapExtractError tags a transient model failure, such as throttling, as ErrRetryable so a caller
// can redrive it, while a terminal failure is returned as an ordinary error. This keeps the model's
// own retryable signal from leaking past the extract seam.
func wrapExtractError(err error) error {
	var retry *llm.RetryableError
	if errors.As(err, &retry) {
		return fmt.Errorf("bedrock extract: %w: %w", ErrRetryable, err)
	}
	return fmt.Errorf("bedrock extract: %w", err)
}

// fileBlock wraps the bytes as the content part that fits the content type, decoding office
// documents to text so the model reads their content instead of the raw zip.
func fileBlock(content []byte, contentType string) llm.Part {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return llm.Image(contentType, content)
	case contentType == "application/pdf":
		return llm.Document(content)
	case isOffice(contentType):
		return llm.Text(officeContent(content))
	default:
		return llm.Text(string(content))
	}
}

// officeContent decodes an office document to text, or a placeholder when it has none, so the
// model still gets a non-empty turn and returns valid JSON.
func officeContent(content []byte) string {
	text, err := officeText(content)
	if err != nil || strings.TrimSpace(text) == "" {
		return "(no readable text in this document)"
	}
	return text
}
