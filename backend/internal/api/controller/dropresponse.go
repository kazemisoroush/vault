package controller

import "github.com/kazemisoroush/vault/backend/internal/domain"

// dropResponse is the body returned by a POST /files call.
type dropResponse struct {
	File      domain.File `json:"file"`
	UploadURL string      `json:"uploadUrl"`
}
