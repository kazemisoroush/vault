package blob

import (
	"context"
	"fmt"
	"io"

	"golang.org/x/oauth2"
	driveapi "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// DriveStorage implements Storage backed by the Google Drive API.
type DriveStorage struct {
	service *driveapi.Service
}

// NewDriveStorage creates a Drive-backed Storage from an OAuth2 token source.
func NewDriveStorage(ctx context.Context, tokenSource oauth2.TokenSource) (*DriveStorage, error) {
	service, err := driveapi.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("creating drive service: %w", err)
	}
	return &DriveStorage{service: service}, nil
}

// ListFiles lists files in the user's Drive, optionally filtered by folder ID.
func (s *DriveStorage) ListFiles(ctx context.Context, folderID string, pageToken string, pageSize int64) ([]Object, string, error) {
	query := "trashed = false"
	if folderID != "" {
		query = fmt.Sprintf("'%s' in parents and trashed = false", folderID)
	}

	call := s.service.Files.List().
		Context(ctx).
		Q(query).
		Fields("nextPageToken, files(id, name, mimeType, size, parents, createdTime, modifiedTime, thumbnailLink, webViewLink)").
		PageSize(pageSize)

	if pageToken != "" {
		call = call.PageToken(pageToken)
	}

	result, err := call.Do()
	if err != nil {
		return nil, "", fmt.Errorf("listing files: %w", err)
	}

	files := make([]Object, 0, len(result.Files))
	for _, f := range result.Files {
		files = append(files, Object{
			ID:           f.Id,
			Name:         f.Name,
			MimeType:     f.MimeType,
			Size:         f.Size,
			Parents:      f.Parents,
			CreatedTime:  f.CreatedTime,
			ModifiedTime: f.ModifiedTime,
			ThumbnailURL: f.ThumbnailLink,
			WebViewURL:   f.WebViewLink,
		})
	}

	return files, result.NextPageToken, nil
}

// UploadFile uploads a file to Google Drive.
func (s *DriveStorage) UploadFile(ctx context.Context, name string, mimeType string, folderID string, reader io.Reader) (*Object, error) {
	driveFile := &driveapi.File{
		Name:     name,
		MimeType: mimeType,
	}
	if folderID != "" {
		driveFile.Parents = []string{folderID}
	}

	result, err := s.service.Files.Create(driveFile).
		Context(ctx).
		Media(reader).
		Fields("id, name, mimeType, size, parents, createdTime, modifiedTime, thumbnailLink, webViewLink").
		Do()
	if err != nil {
		return nil, fmt.Errorf("uploading file: %w", err)
	}

	return &Object{
		ID:           result.Id,
		Name:         result.Name,
		MimeType:     result.MimeType,
		Size:         result.Size,
		Parents:      result.Parents,
		CreatedTime:  result.CreatedTime,
		ModifiedTime: result.ModifiedTime,
		ThumbnailURL: result.ThumbnailLink,
		WebViewURL:   result.WebViewLink,
	}, nil
}

// DownloadFile returns a reader for a file's content.
func (s *DriveStorage) DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	resp, err := s.service.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		return nil, fmt.Errorf("downloading file: %w", err)
	}
	return resp.Body, nil
}

// DeleteFile moves a file to trash in Google Drive.
func (s *DriveStorage) DeleteFile(ctx context.Context, fileID string) error {
	_, err := s.service.Files.Update(fileID, &driveapi.File{Trashed: true}).
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("deleting file: %w", err)
	}
	return nil
}

// GetFile retrieves metadata for a single file.
func (s *DriveStorage) GetFile(ctx context.Context, fileID string) (*Object, error) {
	f, err := s.service.Files.Get(fileID).
		Context(ctx).
		Fields("id, name, mimeType, size, parents, createdTime, modifiedTime, thumbnailLink, webViewLink").
		Do()
	if err != nil {
		return nil, fmt.Errorf("getting file: %w", err)
	}

	return &Object{
		ID:           f.Id,
		Name:         f.Name,
		MimeType:     f.MimeType,
		Size:         f.Size,
		Parents:      f.Parents,
		CreatedTime:  f.CreatedTime,
		ModifiedTime: f.ModifiedTime,
		ThumbnailURL: f.ThumbnailLink,
		WebViewURL:   f.WebViewLink,
	}, nil
}
