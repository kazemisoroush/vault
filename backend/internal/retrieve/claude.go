package retrieve

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// instruction tells the model to act as the vault's search index and return only ids.
const instruction = `You are the search index for a personal file vault.
You are given a JSON catalog of files (id, name, free-form meta, createdAt) and a request.
Return ONLY a JSON array of the ids that match the request, most relevant first.
Match on meaning and on time, for example "last month" against createdAt. Return [] if none match. No commentary.`

// maxTokens caps the reply to a list of ids.
const maxTokens = 1024

// ClaudeRetriever matches files using Claude on Amazon Bedrock, the same model the extractor uses.
type ClaudeRetriever struct {
	model *llm.Model
}

// NewClaudeRetriever builds a ClaudeRetriever for a Bedrock region and model.
func NewClaudeRetriever(_ context.Context, region, model string, recorder llm.Recorder) (*ClaudeRetriever, error) {
	return &ClaudeRetriever{model: llm.NewModel(region, model, "retrieve", recorder)}, nil
}

// Match asks the model which catalog ids satisfy the query.
func (r *ClaudeRetriever) Match(ctx context.Context, query string, files []domain.File) ([]string, error) {
	catalog, err := buildCatalog(files)
	if err != nil {
		return nil, err
	}
	prompt := fmt.Sprintf("Catalog:\n%s\n\nRequest: %s", catalog, query)

	reply, err := r.model.Complete(ctx, instruction+"\n\n"+prompt, maxTokens, anthropic.NewTextBlock(instruction), anthropic.NewTextBlock(prompt))
	if err != nil {
		return nil, fmt.Errorf("bedrock retrieve: %w", err)
	}

	ids, err := parseIDs(reply)
	if err != nil {
		return nil, fmt.Errorf("parse ids: %w", err)
	}
	return ids, nil
}
