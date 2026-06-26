package metadata

import (
	"io"

	"github.com/kazemisoroush/vault/api/internal/model"
)

// UploadRequest holds the data needed to upload a file.
type UploadRequest struct {
	Name     string
	MimeType string
	FolderID string
	Category model.Category
	Tags     []string
	Content  io.Reader
}

// UpdateRequest holds the data for updating file metadata.
type UpdateRequest struct {
	Category *model.Category `json:"category,omitempty"`
	Tags     []string        `json:"tags,omitempty"`
}
