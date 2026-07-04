package controller

import (
	"net/http"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/index"
)

// Get returns one file record and a presigned download URL.
type Get struct {
	index index.Index
	blobs blob.Store
}

// NewGet builds a Get controller.
func NewGet(idx index.Index, blobs blob.Store) *Get {
	return &Get{index: idx, blobs: blobs}
}

func (c *Get) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	file, ok := lookup(w, r, c.index)
	if !ok {
		return
	}

	downloadURL, err := c.blobs.PresignGet(r.Context(), file.Key, presignExpiry)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not presign download")
		return
	}

	writeJSON(w, http.StatusOK, getResponse{File: file, DownloadURL: downloadURL})
}
