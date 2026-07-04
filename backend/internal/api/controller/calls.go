package controller

import (
	"context"
	"net/http"
	"strconv"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

//go:generate go tool mockgen -source=calls.go -destination=../../mocks/calllister_mock.go -package=mocks

// recentCalls bounds how many calls the trace returns.
const recentCalls = int32(50)

// CallLister lists recent LLM calls for the trace view.
type CallLister interface {
	List(ctx context.Context, limit int32) ([]llm.Call, error)
}

// CallsController serves the recent LLM call trace.
type CallsController struct {
	lister CallLister
}

// NewCallsController builds a CallsController.
func NewCallsController(lister CallLister) *CallsController {
	return &CallsController{lister: lister}
}

type callsResponse struct {
	Calls []llm.Call `json:"calls"`
}

// Calls returns the most recent LLM calls, newest first.
func (c *CallsController) Calls(w http.ResponseWriter, r *http.Request) {
	limit := recentCalls
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = min(int32(parsed), recentCalls)
	}

	list, err := c.lister.List(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read the call log")
		return
	}
	writeJSON(w, http.StatusOK, callsResponse{Calls: list})
}
