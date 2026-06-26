package drive

import (
	"context"
	"io"
)

// Service defines the interface for Drive operations.
type Service interface {
	ListFiles(ctx context.Context, folderID string, pageToken string, pageSize int64) ([]DriveFile, string, error)
	UploadFile(ctx context.Context, name string, mimeType string, folderID string, reader io.Reader) (*DriveFile, error)
	DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error)
	DeleteFile(ctx context.Context, fileID string) error
	GetFile(ctx context.Context, fileID string) (*DriveFile, error)
}
