package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/embed"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/retrieve"
	"github.com/kazemisoroush/vault/backend/internal/vectors"
)

// shortlistSize bounds the nearest files pulled from the vector store before the model re-ranks them.
const shortlistSize = int32(20)

// AskController answers a natural-language query with the matching files.
type AskController struct {
	index     index.Index
	blobs     blob.Store
	embedder  embed.Embedder
	vectors   vectors.Store
	retriever retrieve.Retriever
}

// NewAskController builds an AskController.
func NewAskController(idx index.Index, blobs blob.Store, embedder embed.Embedder, store vectors.Store, retriever retrieve.Retriever) *AskController {
	return &AskController{index: idx, blobs: blobs, embedder: embedder, vectors: store, retriever: retriever}
}

type askRequest struct {
	Query string `json:"query"`
}

type askResult struct {
	File        domain.File `json:"file"`
	DownloadURL string      `json:"downloadUrl"`
}

type askResponse struct {
	Answer  string      `json:"answer,omitempty"`
	Results []askResult `json:"results"`
}

// Ask embeds the query, pulls the nearest files from the vector store, lets the model re-rank
// that shortlist, and presigns a download for each match. Cost is constant in the vault size.
func (c *AskController) Ask(w http.ResponseWriter, r *http.Request) {
	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	vector, err := c.embedder.Embed(r.Context(), req.Query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not embed the query")
		return
	}

	nearest, err := c.vectors.Query(r.Context(), auth.Owner(r.Context()), vector, shortlistSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not search the vectors")
		return
	}

	shortlist := c.load(r.Context(), nearest)

	answer, err := c.retriever.Match(r.Context(), req.Query, shortlist)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not run the search")
		return
	}

	byID := make(map[string]domain.File, len(shortlist))
	for _, file := range shortlist {
		byID[file.ID] = file
	}

	results := make([]askResult, 0, len(answer.IDs))
	for _, id := range answer.IDs {
		file, ok := byID[id]
		if !ok {
			continue
		}
		downloadURL, err := c.blobs.PresignGet(r.Context(), file.Key, presignExpiry)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not presign download")
			return
		}
		results = append(results, askResult{File: file, DownloadURL: downloadURL})
	}

	writeJSON(w, http.StatusOK, askResponse{Answer: answer.Text, Results: results})
}

// load fetches the records for the owner-scoped shortlist the vector store returned.
func (c *AskController) load(ctx context.Context, ids []string) []domain.File {
	files := make([]domain.File, 0, len(ids))
	for _, id := range ids {
		file, err := c.index.Get(ctx, id)
		if err != nil {
			continue
		}
		files = append(files, file)
	}
	return files
}
