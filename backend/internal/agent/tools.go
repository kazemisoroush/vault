package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// Tool names the model calls.
const (
	toolSearchByMeaning = "search_by_meaning"
	toolGetFile         = "get_file"
)

// defaultFileLimit and maxFileLimit bound how many passages a search returns.
const (
	defaultFileLimit = 20
	maxFileLimit     = 50
)

// tools declares the query tools the model may call.
func tools() []llm.Tool {
	return []llm.Tool{
		{
			Name: toolSearchByMeaning,
			Description: "Search the vault by hybrid meaning and keyword. Give the text to search for " +
				"and an optional limit. Include exact terms such as ids, numbers, or names verbatim. Returns " +
				"passages from the matching files, each with its file id and name; cite the file id in your answer.",
			Schema: map[string]any{
				"query": map[string]any{"type": "string", "description": "what to look for, including any exact terms verbatim"},
				"limit": map[string]any{"type": "integer", "description": "max passages to return"},
			},
			Required: []string{"query"},
		},
		{
			Name:        toolGetFile,
			Description: "Read one file's record by its id.",
			Schema:      map[string]any{"id": map[string]any{"type": "string"}},
			Required:    []string{"id"},
		},
	}
}

// executor returns the function that runs whichever tool the model called, always scoped to owner.
func (a *QuestionAnswerer) executor(ownerID string) llm.ToolExecutor {
	return func(ctx context.Context, call llm.ToolCall) (string, error) {
		switch call.Name {
		case toolSearchByMeaning:
			return a.runSearch(ctx, ownerID, call.Input)
		case toolGetFile:
			return a.runGet(ctx, ownerID, call.Input)
		default:
			return "", fmt.Errorf("unknown tool %q", call.Name)
		}
	}
}

// passageView is the compact view of a retrieved passage the model reads: the text so it can reason
// over the content, and the file it came from so it can cite the file id.
type passageView struct {
	FileID   string `json:"fileId"`
	FileName string `json:"fileName,omitempty"`
	Text     string `json:"text"`
}

type searchInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// runSearch retrieves the passages most relevant to the query by hybrid search and returns them for
// the model to reason over. The managed Knowledge Base is one shared store that carries no owner
// metadata yet, so retrieval is not owner-scoped on its own; every passage is dropped unless its
// file belongs to ownerID, keeping another owner's text from ever reaching the model.
func (a *QuestionAnswerer) runSearch(ctx context.Context, ownerID string, raw json.RawMessage) (string, error) {
	var in searchInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("decode %s input: %w", toolSearchByMeaning, err)
	}
	passages, err := a.searcher.Search(ctx, in.Query, clampLimit(in.Limit))
	if err != nil {
		return "", fmt.Errorf("retrieve: %w", err)
	}
	owned := make(map[string]bool, len(passages))
	views := make([]passageView, 0, len(passages))
	for _, passage := range passages {
		allow, seen := owned[passage.FileID]
		if !seen {
			file, err := a.index.Get(ctx, passage.FileID)
			allow = err == nil && file.OwnerID == ownerID
			owned[passage.FileID] = allow
		}
		if !allow {
			continue
		}
		views = append(views, passageView{FileID: passage.FileID, FileName: passage.FileName, Text: passage.Text})
	}
	data, err := json.Marshal(views)
	if err != nil {
		return "", fmt.Errorf("marshal search results: %w", err)
	}
	return string(data), nil
}

type getInput struct {
	ID string `json:"id"`
}

// runGet reads one file the owner owns. A missing or foreign file returns a not-found result so the
// model learns the outcome without another owner's file ever leaking.
func (a *QuestionAnswerer) runGet(ctx context.Context, ownerID string, raw json.RawMessage) (string, error) {
	var in getInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("decode %s input: %w", toolGetFile, err)
	}
	file, err := a.index.Get(ctx, in.ID)
	if err != nil || file.OwnerID != ownerID {
		return `{"error":"file not found"}`, nil
	}
	view := fileView{ID: file.ID, Name: file.Name, Meta: file.Meta, CreatedAt: file.CreatedAt.Format(time.RFC3339)}
	data, err := json.Marshal(view)
	if err != nil {
		return "", fmt.Errorf("marshal file result: %w", err)
	}
	return string(data), nil
}

// fileView is the compact view of a file the model reads from get_file.
type fileView struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Meta      map[string]string `json:"meta,omitempty"`
	CreatedAt string            `json:"createdAt"`
}

// clampLimit keeps a requested limit within sane bounds, defaulting when unset.
func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultFileLimit
	}
	if limit > maxFileLimit {
		return maxFileLimit
	}
	return limit
}
