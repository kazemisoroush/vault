package extract

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// instruction tells the model to return only a flat JSON metadata object, kept in its own file
// so it can be read, edited, and evaluated on its own. It is used for files whose text is
// already decoded deterministically, so the model only fills the metadata.
//
//go:embed prompts/meta.prompt
var instruction string

// transcribeInstruction asks for metadata and a full transcription in one reply, kept in its own
// file like the metadata prompt. It is used for images and PDFs, whose text the backend cannot
// decode itself. The transcription becomes the file's canonical text, so it must be word-for-word.
//
//go:embed prompts/transcribe.prompt
var transcribeInstruction string

// maxTokens caps the model reply to the size of a small flat metadata object, and
// transcribeMaxTokens leaves room for a full document transcription beside it.
const (
	maxTokens           = 1024
	transcribeMaxTokens = 8192
)

// ClaudeExtractor extracts metadata using Claude on Amazon Bedrock.
type ClaudeExtractor struct {
	model *llm.Model
}

// NewClaudeExtractor builds a ClaudeExtractor for a Bedrock region and model.
func NewClaudeExtractor(_ context.Context, region, model string, recorder llm.Recorder) (*ClaudeExtractor, error) {
	return &ClaudeExtractor{model: llm.NewModel(region, model, "extract", recorder)}, nil
}

// Extract sends the file to the model and returns its metadata and canonical text. Text-bearing
// formats keep their deterministically decoded text; images and PDFs are transcribed by the model
// in the same call that extracts their metadata.
func (e *ClaudeExtractor) Extract(ctx context.Context, content []byte, contentType string) (Extraction, error) {
	if needsTranscription(contentType) {
		extraction, err := e.extractTranscribing(ctx, content, contentType)
		if err != nil {
			return Extraction{}, fmt.Errorf("extract with transcription: %w", err)
		}
		return extraction, nil
	}

	text := deterministicText(content, contentType)
	prompt := fmt.Sprintf("%s\n\n[file: %s, %d bytes]", instruction, contentType, len(content))
	reply, err := e.model.Converse(ctx, llm.Conversation{
		Prompt:    prompt,
		Content:   []llm.Part{fileBlock(content, contentType), llm.Text(instruction)},
		MaxTokens: maxTokens,
	})
	if err != nil {
		return Extraction{}, wrapExtractError(err)
	}

	// Merge the model's metadata over the file's own embedded metadata, treating a declined reply as none.
	result := embeddedMeta(content, contentType)
	maps.Copy(result, metaFromReply(reply))
	return Extraction{Meta: result, Text: text}, nil
}

// extractTranscribing asks the model for metadata and a word-for-word transcription in one call.
// When the combined reply does not parse (typically a transcription truncated at the token cap),
// it falls back to a metadata-only call so a long document still lands with searchable metadata;
// only the stored text is given up, and a re-drop can retry it.
func (e *ClaudeExtractor) extractTranscribing(ctx context.Context, content []byte, contentType string) (Extraction, error) {
	prompt := fmt.Sprintf("%s\n\n[file: %s, %d bytes]", transcribeInstruction, contentType, len(content))
	reply, err := e.model.Converse(ctx, llm.Conversation{
		Prompt:    prompt,
		Content:   []llm.Part{fileBlock(content, contentType), llm.Text(transcribeInstruction)},
		MaxTokens: transcribeMaxTokens,
	})
	if err != nil {
		return Extraction{}, wrapExtractError(err)
	}

	meta, text, ok := transcriptionFromReply(reply)
	if !ok {
		extraction, err := e.extractMetaOnly(ctx, content, contentType)
		if err != nil {
			return Extraction{}, fmt.Errorf("metadata-only fallback: %w", err)
		}
		return extraction, nil
	}
	result := embeddedMeta(content, contentType)
	maps.Copy(result, meta)
	return Extraction{Meta: result, Text: text}, nil
}

// extractMetaOnly is the transcription fallback: the plain metadata call, with no stored text.
func (e *ClaudeExtractor) extractMetaOnly(ctx context.Context, content []byte, contentType string) (Extraction, error) {
	prompt := fmt.Sprintf("%s\n\n[file: %s, %d bytes]", instruction, contentType, len(content))
	reply, err := e.model.Converse(ctx, llm.Conversation{
		Prompt:    prompt,
		Content:   []llm.Part{fileBlock(content, contentType), llm.Text(instruction)},
		MaxTokens: maxTokens,
	})
	if err != nil {
		return Extraction{}, wrapExtractError(err)
	}

	result := embeddedMeta(content, contentType)
	maps.Copy(result, metaFromReply(reply))
	return Extraction{Meta: result}, nil
}

// needsTranscription reports whether the model must transcribe the file's text because the
// backend cannot decode it deterministically.
func needsTranscription(contentType string) bool {
	return strings.HasPrefix(contentType, "image/") || contentType == "application/pdf"
}

// deterministicText decodes the file's text without a model: office documents through their
// package structure, everything else as plain bytes. An office document with no readable text
// yields an empty string, so the model-turn placeholder never becomes stored canonical text.
func deterministicText(content []byte, contentType string) string {
	if isOffice(contentType) {
		text, err := officeText(content)
		if err != nil || strings.TrimSpace(text) == "" {
			return ""
		}
		return text
	}
	return string(content)
}

// wrapExtractError tags a transient model failure, such as throttling, as ErrRetryable so a caller
// can redrive it, while a terminal failure is returned as an ordinary error. The model's own error
// is kept only as text (%v, not %w), so its type does not leak past the extract seam; callers match
// on ErrRetryable alone.
func wrapExtractError(err error) error {
	var retry *llm.RetryableError
	if errors.As(err, &retry) {
		return fmt.Errorf("bedrock extract: %w: %v", ErrRetryable, err)
	}
	return fmt.Errorf("bedrock extract: %w", err)
}

// fileBlock wraps the bytes as the content part that fits the content type, decoding office
// documents to text so the model reads their content instead of the raw zip.
func fileBlock(content []byte, contentType string) llm.Part {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return imageBlock(content, contentType)
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
