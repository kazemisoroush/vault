package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// Tool names the model calls.
const (
	toolSearchByMeaning = "search_by_meaning"
	toolFindByFacts     = "find_by_facts"
	toolGetFile         = "get_file"
)

// defaultFileLimit and maxFileLimit bound how many files a tool returns.
const (
	defaultFileLimit = 20
	maxFileLimit     = 50
	listPageSize     = int32(100)
)

// tools declares the query tools the model may call.
func tools() []llm.Tool {
	return []llm.Tool{
		{
			Name:        toolSearchByMeaning,
			Description: "Find files by meaning. Give the text to search for and an optional limit.",
			Schema: map[string]any{
				"query": map[string]any{"type": "string", "description": "what to look for, in words"},
				"limit": map[string]any{"type": "integer", "description": "max files to return"},
			},
			Required: []string{"query"},
		},
		{
			Name: toolFindByFacts,
			Description: "Find files by their facts. A file matches when every pair's field contains its " +
				"value, ignoring case. field is a metadata key or \"name\". Optionally bound the file's " +
				"created time with since and until, each an RFC3339 timestamp or a plain YYYY-MM-DD date.",
			Schema: map[string]any{
				"contains": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"field": map[string]any{"type": "string"},
							"value": map[string]any{"type": "string"},
						},
						"required": []string{"field", "value"},
					},
				},
				"since": map[string]any{"type": "string", "description": "only files created on or after this RFC3339 date"},
				"until": map[string]any{"type": "string", "description": "only files created on or before this RFC3339 date"},
				"limit": map[string]any{"type": "integer"},
			},
		},
		{
			Name:        toolGetFile,
			Description: "Read one file by its id.",
			Schema:      map[string]any{"id": map[string]any{"type": "string"}},
			Required:    []string{"id"},
		},
	}
}

// executor returns the function that runs whichever tool the model called, always scoped to owner.
func (a *Agent) executor(ownerID string) llm.ToolExecutor {
	return func(ctx context.Context, call llm.ToolCall) (string, error) {
		switch call.Name {
		case toolSearchByMeaning:
			return a.runSearch(ctx, ownerID, call.Input)
		case toolFindByFacts:
			return a.runFacts(ctx, ownerID, call.Input)
		case toolGetFile:
			return a.runGet(ctx, ownerID, call.Input)
		default:
			return "", fmt.Errorf("unknown tool %q", call.Name)
		}
	}
}

// fileView is the compact view of a file the model reads in a tool result.
type fileView struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Meta      map[string]string `json:"meta,omitempty"`
	CreatedAt string            `json:"createdAt"`
}

type searchInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// runSearch embeds the query, pulls the owner's nearest files, and returns them as views.
func (a *Agent) runSearch(ctx context.Context, ownerID string, raw json.RawMessage) (string, error) {
	var in searchInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("decode %s input: %w", toolSearchByMeaning, err)
	}
	vector, err := a.embedder.Embed(ctx, in.Query)
	if err != nil {
		return "", fmt.Errorf("embed query: %w", err)
	}
	ids, err := a.vectors.Query(ctx, ownerID, vector, int32(clampLimit(in.Limit)))
	if err != nil {
		return "", fmt.Errorf("query vectors: %w", err)
	}
	return marshalViews(a.load(ctx, ownerID, ids))
}

type predicate struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type factsInput struct {
	Contains []predicate `json:"contains"`
	Since    string      `json:"since"`
	Until    string      `json:"until"`
	Limit    int         `json:"limit"`
}

// runFacts scans the owner's files and returns those that pass every predicate and time bound.
func (a *Agent) runFacts(ctx context.Context, ownerID string, raw json.RawMessage) (string, error) {
	var in factsInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("decode %s input: %w", toolFindByFacts, err)
	}
	since := parseDate(in.Since)
	until := parseDate(in.Until)
	limit := clampLimit(in.Limit)

	matches := make([]domain.File, 0, limit)
	cursor := ""
	for {
		page, next, err := a.index.List(ctx, ownerID, listPageSize, cursor)
		if err != nil {
			return "", fmt.Errorf("list owner files: %w", err)
		}
		for _, file := range page {
			if matchesFacts(file, in.Contains, since, until) {
				matches = append(matches, file)
				if len(matches) >= limit {
					return marshalViews(matches)
				}
			}
		}
		if next == "" {
			break
		}
		cursor = next
	}
	return marshalViews(matches)
}

type getInput struct {
	ID string `json:"id"`
}

// runGet reads one file the owner owns. A missing or foreign file returns a not-found result so
// the model learns the outcome without another owner's file ever leaking.
func (a *Agent) runGet(ctx context.Context, ownerID string, raw json.RawMessage) (string, error) {
	var in getInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("decode %s input: %w", toolGetFile, err)
	}
	file, err := a.index.Get(ctx, in.ID)
	if err != nil || file.OwnerID != ownerID {
		return `{"error":"file not found"}`, nil
	}
	return marshalViews([]domain.File{file})
}

// matchesFacts reports whether a file passes every predicate and the time bounds. A predicate
// matches when the named metadata key, or the name, contains the value without regard to case.
func matchesFacts(file domain.File, predicates []predicate, since, until time.Time) bool {
	for _, want := range predicates {
		if !fieldContains(file, want.Field, want.Value) {
			return false
		}
	}
	if !since.IsZero() && file.CreatedAt.Before(since) {
		return false
	}
	if !until.IsZero() && file.CreatedAt.After(until) {
		return false
	}
	return true
}

// fieldContains reports whether the file's field holds value, ignoring case. The field is a
// metadata key, or "name" for the file name.
func fieldContains(file domain.File, field, value string) bool {
	var have string
	if strings.EqualFold(field, "name") {
		have = file.Name
	} else {
		have = file.Meta[field]
		if have == "" {
			have = metaValueFold(file.Meta, field)
		}
	}
	return strings.Contains(strings.ToLower(have), strings.ToLower(value))
}

// metaValueFold looks up a metadata key without regard to case, so "Vendor" finds "vendor".
func metaValueFold(meta map[string]string, field string) string {
	for key, value := range meta {
		if strings.EqualFold(key, field) {
			return value
		}
	}
	return ""
}

// marshalViews renders files as the compact JSON the model reads.
func marshalViews(files []domain.File) (string, error) {
	views := make([]fileView, 0, len(files))
	for _, file := range files {
		views = append(views, fileView{
			ID:        file.ID,
			Name:      file.Name,
			Meta:      file.Meta,
			CreatedAt: file.CreatedAt.Format(time.RFC3339),
		})
	}
	data, err := json.Marshal(views)
	if err != nil {
		return "", fmt.Errorf("marshal file views: %w", err)
	}
	return string(data), nil
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

// dateLayouts are the date formats a since or until bound may use: a full RFC3339 timestamp, or a
// plain calendar date, since the model often gives the latter.
var dateLayouts = []string{time.RFC3339, "2006-01-02"}

// parseDate reads a date bound in any of the accepted layouts, returning the zero time when empty
// or unparseable, which the caller treats as no bound.
func parseDate(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	for _, layout := range dateLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}
