package vision

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// transcribePrompt asks the model to transcribe the file word for word plus a searchable summary,
// kept in its own file so it can be read and tuned on its own.
//
//go:embed prompt.txt
var transcribePrompt string

// transcribeMaxTokens leaves room for a full document transcription.
const transcribeMaxTokens = 8192

// ClaudeTranscriber transcribes images and PDFs to searchable text with Claude on Bedrock.
type ClaudeTranscriber struct {
	model *llm.Model
}

// NewClaudeTranscriber builds a ClaudeTranscriber for a Bedrock region and model.
func NewClaudeTranscriber(region string, model string, recorder llm.Recorder) *ClaudeTranscriber {
	return &ClaudeTranscriber{model: llm.NewModel(region, model, "transcribe", recorder)}
}

// Transcribe reads the file with the model and returns its text, so an image or scanned PDF becomes
// searchable in the Knowledge Base. An oversized image is downscaled to fit the per-image limit.
func (t *ClaudeTranscriber) Transcribe(ctx context.Context, content []byte, contentType string) (string, error) {
	text, err := t.model.Converse(ctx, llm.Conversation{
		Content:   []llm.Part{fileBlock(content, contentType), llm.Text(transcribePrompt)},
		MaxTokens: transcribeMaxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("transcribe %s: %w", contentType, err)
	}
	return text, nil
}
