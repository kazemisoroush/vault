package controller

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/kazemisoroush/vault/backend/internal/agent"
	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
)

// AskController answers a natural-language query with an answer and the files it used.
type AskController struct {
	agent agent.Answerer
	blobs blob.Store
}

// NewAskController builds an AskController.
func NewAskController(answerer agent.Answerer, blobs blob.Store) *AskController {
	return &AskController{agent: answerer, blobs: blobs}
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

// Ask hands the query to the agent, which writes and runs its own queries over the owner's vault,
// then presigns a download for each file the agent used so the caller gets the answer and its
// sources as openable links.
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

	ownerID := auth.OwnerID(r.Context())
	result, err := c.agent.Answer(r.Context(), ownerID, req.Query)
	if err != nil {
		log.Printf("ask: answer: %v", err)
		writeError(w, http.StatusInternalServerError, "could not answer the query")
		return
	}

	results := make([]askResult, 0, len(result.Files))
	for _, file := range result.Files {
		downloadURL, err := c.blobs.PresignGet(r.Context(), file.Key, presignExpiry)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not presign download")
			return
		}
		results = append(results, askResult{File: file, DownloadURL: downloadURL})
	}

	writeJSON(w, http.StatusOK, askResponse{Answer: result.Text, Results: results})
}
