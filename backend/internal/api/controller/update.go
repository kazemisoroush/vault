package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kazemisoroush/vault/backend/internal/index"
)

// Update changes a file's name or free-form metadata.
type Update struct {
	index index.Index
	now   func() time.Time
}

// NewUpdate builds an Update controller with a real clock.
func NewUpdate(idx index.Index) *Update {
	return &Update{index: idx, now: time.Now}
}

func (c *Update) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	file, ok := lookup(w, r, c.index)
	if !ok {
		return
	}

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == nil && req.Meta == nil {
		writeError(w, http.StatusBadRequest, "nothing to update")
		return
	}

	if req.Name != nil {
		if *req.Name == "" {
			writeError(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		file.Name = *req.Name
	}
	if req.Meta != nil {
		file.Meta = *req.Meta
	}
	file.UpdatedAt = c.now().UTC()

	if err := c.index.Put(r.Context(), file); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save file record")
		return
	}

	writeJSON(w, http.StatusOK, file)
}
