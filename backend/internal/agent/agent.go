// Package agent answers a natural-language query over the vault. It lets the model write and run
// queries through a few owner-scoped tools, then returns the answer with the files it used.
package agent

import (
	"context"
	"fmt"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/embed"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/llm"
	"github.com/kazemisoroush/vault/backend/internal/vectors"
)

//go:generate go tool mockgen -source=agent.go -destination=mock/agent_mock.go -package=agentmock

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

// Answerer answers a query for one owner. The controller depends on this, not the concrete agent.
type Answerer interface {
	Answer(ctx context.Context, ownerID, query string) (Result, error)
}

// Converser runs one tool-using exchange with the model. The concrete *llm.Model satisfies it.
type Converser interface {
	Converse(ctx context.Context, c llm.Conversation) (string, error)
}

// Agent answers queries by driving the model over the vault's stores through its tools.
type Agent struct {
	model    Converser
	embedder embed.Embedder
	vectors  vectors.Store
	index    index.Index
}

// New builds an Agent over the model and the stores that already serve the vault.
func New(model Converser, embedder embed.Embedder, store vectors.Store, idx index.Index) *Agent {
	return &Agent{model: model, embedder: embedder, vectors: store, index: idx}
}

// Answer lets the model query the owner's vault through the tools and returns the answer with the
// files it used. Every store call the tools make is scoped to ownerID, which the model cannot set.
func (a *Agent) Answer(ctx context.Context, ownerID, query string) (Result, error) {
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

	answer, ids := parseFinal(reply)
	return Result{Text: answer, Files: a.load(ctx, ownerID, ids)}, nil
}

// load fetches the owner's records for the ids the model cited, skipping any it does not own or
// that no longer exist, so a stale citation never leaks another owner's file.
func (a *Agent) load(ctx context.Context, ownerID string, ids []string) []domain.File {
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

// systemPrompt tells the model how to use the tools and how to shape its final reply.
const systemPrompt = `You are a personal file vault assistant. Answer the user's request about their files.
You have tools to find files: search_by_meaning for fuzzy questions, find_by_facts to filter by a
metadata value or a date range, and get_file to read one file by id. Use them as needed, then stop.
Reply with ONLY a JSON object: {"answer": string, "ids": [string]}.
- ids: the file ids you used to answer, most relevant first, or [] if none fit.
- answer: a short, direct answer drawn only from the files you found, or "" for a plain find.
No markdown, no commentary, only the JSON object.`
