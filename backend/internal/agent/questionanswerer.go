// Package agent answers a natural-language query over the vault. It lets the model write and run
// queries through a few owner-scoped tools, then returns the answer with the files it used.
package agent

import (
	"context"
	"fmt"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/kb"
	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// ModelOp is the operation label the agent's model calls are tagged with on the trace.
const ModelOp = "agent"

// answerMaxTokens caps each model reply during the exchange.
const answerMaxTokens = 1024

// maxRounds caps the model calls in one answer. It leaves room to search, look, and answer, and
// stops a model that keeps calling tools from looping forever.
const maxRounds = 6

// Result is the agent's output: a human-readable answer and the files it used to reach it.
type Result struct {
	Text  string
	Files []domain.File
}

// QuestionAnswerer answers queries by driving the model over the vault through its tools: hybrid search over
// the Knowledge Base, and a read of one file's record.
type QuestionAnswerer struct {
	model    Converser
	searcher kb.Searcher
	index    index.Index
}

// NewQuestionAnswerer builds a QuestionAnswerer over the model, the Knowledge Base searcher, and the file index.
func NewQuestionAnswerer(model Converser, s kb.Searcher, idx index.Index) *QuestionAnswerer {
	return &QuestionAnswerer{model: model, searcher: s, index: idx}
}

// Answer lets the model query the owner's vault through the tools and returns the answer with the
// files it used. Every store call the tools make is scoped to ownerID, which the model cannot set.
func (a *QuestionAnswerer) Answer(ctx context.Context, ownerID, query string) (Result, error) {
	reply, err := a.model.Converse(ctx, llm.Conversation{
		System:    systemPrompt,
		Prompt:    query,
		MaxTokens: answerMaxTokens,
		Tools:     tools(),
		Execute:   a.executor(ownerID),
		MaxRounds: maxRounds,
	})
	if err != nil {
		return Result{}, fmt.Errorf("agent converse: %w", err)
	}

	answer, fileIDs := parseFinal(reply)
	return Result{Text: answer, Files: a.load(ctx, ownerID, fileIDs)}, nil
}

// load fetches the owner's records for the ids the model cited, skipping any it does not own or
// that no longer exist, so a stale citation never leaks another owner's file.
func (a *QuestionAnswerer) load(ctx context.Context, ownerID string, ids []string) []domain.File {
	files := make([]domain.File, 0, len(ids))
	for _, id := range ids {
		file, err := a.index.Get(ctx, id)
		if err != nil || file.OwnerID != ownerID {
			continue
		}
		files = append(files, file)
	}
	return files
}

// systemPrompt lives in prompt.go, rendered from the embedded template and the declared tools.
