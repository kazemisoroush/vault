package blob

import (
	"context"
	"io"
)

//go:generate go tool mockgen -source=storage.go -destination=storage_mock.go -package=blob

// Storage defines the interface for blob storage operations.
type Storage interface {
	ListFiles(ctx context.Context, folderID string, pageToken string, pageSize int64) ([]Object, string, error)
	UploadFile(ctx context.Context, name string, mimeType string, folderID string, reader io.Reader) (*Object, error)
	DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error)
	DeleteFile(ctx context.Context, fileID string) error
	GetFile(ctx context.Context, fileID string) (*Object, error)
}

// Object holds raw blob metadata.
type Object struct {
	ID           string
	Name         string
	MimeType     string
	Size         int64
	Parents      []string
	CreatedTime  string
	ModifiedTime string
	ThumbnailURL string
	WebViewURL   string
}
