package controller

import (
	"net/http"
	"strconv"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
)

// List returns one page of file records.
type List struct {
	index index.Index
}

// NewList builds a List controller.
func NewList(idx index.Index) *List {
	return &List{index: idx}
}

type listResponse struct {
	Files  []domain.File `json:"files"`
	Cursor string        `json:"cursor,omitempty"`
}

func (c *List) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	limit := defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = min(int32(parsed), maxLimit)
	}

	files, cursor, err := c.index.List(r.Context(), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list files")
		return
	}

	writeJSON(w, http.StatusOK, listResponse{Files: files, Cursor: cursor})
}
