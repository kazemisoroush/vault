package controller

import "github.com/kazemisoroush/vault/backend/internal/domain"

// getResponse is the body returned by a GET /files/{id} call.
type getResponse struct {
	File        domain.File `json:"file"`
	DownloadURL string      `json:"downloadUrl"`
}
