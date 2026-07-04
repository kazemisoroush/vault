package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/retrieve"
)

// catalogLimit bounds the files sent to the retriever; a personal vault fits in one page.
const catalogLimit = int32(500)

// AskController answers a natural-language query with the matching files.
type AskController struct {
	index     index.Index
	blobs     blob.Store
	retriever retrieve.Retriever
}

// NewAskController builds an AskController.
func NewAskController(idx index.Index, blobs blob.Store, retriever retrieve.Retriever) *AskController {
	return &AskController{index: idx, blobs: blobs, retriever: retriever}
}

type askRequest struct {
	Query string `json:"query"`
}

type askResult struct {
	File        domain.File `json:"file"`
	DownloadURL string      `json:"downloadUrl"`
}

type askResponse struct {
	Results []askResult `json:"results"`
}

// Ask lists the catalog, asks the retriever which files match, and presigns a download for each.
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

	files, _, err := c.index.List(r.Context(), catalogLimit, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read the index")
		return
	}

	ids, err := c.retriever.Match(r.Context(), req.Query, files)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not run the search")
		return
	}

	byID := make(map[string]domain.File, len(files))
	for _, file := range files {
		byID[file.ID] = file
	}

	results := make([]askResult, 0, len(ids))
	for _, id := range ids {
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

	writeJSON(w, http.StatusOK, askResponse{Results: results})
}
