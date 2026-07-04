package controller

import (
	"net/http"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/index"
)

// Delete removes a file record and its bytes.
type Delete struct {
	index index.Index
	blobs blob.Store
}

// NewDelete builds a Delete controller.
func NewDelete(idx index.Index, blobs blob.Store) *Delete {
	return &Delete{index: idx, blobs: blobs}
}

func (c *Delete) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	file, ok := lookup(w, r, c.index)
	if !ok {
		return
	}

	if err := c.index.Delete(r.Context(), file.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete file record")
		return
	}

	if err := c.blobs.Delete(r.Context(), file.Key); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete file bytes")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
