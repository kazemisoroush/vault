package retrieve

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// instruction tells the model to answer the request and return the matching ids.
const instruction = `You are a personal file vault assistant.
You are given a JSON catalog of files (id, name, free-form meta, createdAt) and a request.
Reply with ONLY a JSON object: {"answer": string, "ids": [string]}.
- ids: the file ids that match, most relevant first; [] if none. Match on meaning and on time, for example "last month" against createdAt.
- answer: a short, direct, human-readable answer to the request, drawn ONLY from the metadata shown. If the request is a plain find, or the metadata does not contain the answer, use "".
No markdown, no commentary, only the JSON object.`

// maxTokens caps the reply to a short answer and a list of ids.
const maxTokens = 1024

// ClaudeRetriever matches files using Claude on Amazon Bedrock, the same model the extractor uses.
type ClaudeRetriever struct {
	model *llm.Model
}

// NewClaudeRetriever builds a ClaudeRetriever for a Bedrock region and model.
func NewClaudeRetriever(_ context.Context, region, model string, recorder llm.Recorder) (*ClaudeRetriever, error) {
	return &ClaudeRetriever{model: llm.NewModel(region, model, "retrieve", recorder)}, nil
}

// Match asks the model to answer the query over the catalog and return the matching ids.
func (r *ClaudeRetriever) Match(ctx context.Context, query string, files []domain.File) (Answer, error) {
	catalog, err := buildCatalog(files)
	if err != nil {
		return Answer{}, err
	}
	prompt := fmt.Sprintf("Catalog:\n%s\n\nRequest: %s", catalog, query)

	reply, err := r.model.Complete(ctx, instruction+"\n\n"+prompt, maxTokens, anthropic.NewTextBlock(instruction), anthropic.NewTextBlock(prompt))
	if err != nil {
		return Answer{}, fmt.Errorf("bedrock retrieve: %w", err)
	}

	answer, err := parseAnswer(reply)
	if err != nil {
		return Answer{}, fmt.Errorf("parse answer: %w", err)
	}
	return answer, nil
}
