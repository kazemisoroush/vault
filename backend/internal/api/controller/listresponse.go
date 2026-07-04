package controller

import "github.com/kazemisoroush/vault/backend/internal/domain"

// listResponse is the body returned by a GET /files call.
type listResponse struct {
	Files  []domain.File `json:"files"`
	Cursor string        `json:"cursor,omitempty"`
}
