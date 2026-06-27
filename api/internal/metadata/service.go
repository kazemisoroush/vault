package metadata

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kazemisoroush/vault/api/internal/blob"
	"github.com/kazemisoroush/vault/api/internal/model"
	"github.com/kazemisoroush/vault/api/internal/storage"
)

// ErrNotFound is returned when a requested file does not exist.
var ErrNotFound = errors.New("file not found")

// Service coordinates blob storage and metadata operations.
type Service struct {
	blobs blob.Storage
	repo  storage.Repository
}

// NewService creates a new metadata service.
func NewService(blobs blob.Storage, repo storage.Repository) *Service {
	return &Service{blobs: blobs, repo: repo}
}

// ListFiles returns files matching the query.
func (s *Service) ListFiles(ctx context.Context, query model.FileQuery) (*model.FileListResult, error) {
	result, err := s.repo.ListFiles(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing files: %w", err)
	}
	return result, nil
}

// GetFile returns a single file's metadata.
func (s *Service) GetFile(ctx context.Context, id string) (*model.File, error) {
	file, err := s.repo.GetFile(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting file: %w", err)
	}
	return file, nil
}

// UploadFile uploads to blob storage and stores metadata.
func (s *Service) UploadFile(ctx context.Context, req UploadRequest) (*model.File, error) {
	obj, err := s.blobs.UploadFile(ctx, req.Name, req.MimeType, req.FolderID, req.Content)
	if err != nil {
		return nil, fmt.Errorf("uploading to blob storage: %w", err)
	}

	now := time.Now()
	file := model.File{
		ID:           uuid.New().String(),
		DriveFileID:  obj.ID,
		Name:         obj.Name,
		MimeType:     obj.MimeType,
		Size:         obj.Size,
		Category:     req.Category,
		Tags:         req.Tags,
		DrivePath:    buildDrivePath(obj.Parents),
		ThumbnailURL: obj.ThumbnailURL,
		WebViewURL:   obj.WebViewURL,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.PutFile(ctx, file); err != nil {
		return nil, fmt.Errorf("storing metadata: %w", err)
	}

	return &file, nil
}

// UpdateFile updates a file's metadata (tags, category).
func (s *Service) UpdateFile(ctx context.Context, id string, req UpdateRequest) (*model.File, error) {
	file, err := s.repo.GetFile(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting file: %w", err)
	}
	if file == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}

	if req.Category != nil {
		file.Category = *req.Category
	}
	if req.Tags != nil {
		file.Tags = req.Tags
	}
	file.UpdatedAt = time.Now()

	if err := s.repo.PutFile(ctx, *file); err != nil {
		return nil, fmt.Errorf("updating metadata: %w", err)
	}

	return file, nil
}

// DeleteFile removes from blob storage and deletes metadata.
func (s *Service) DeleteFile(ctx context.Context, id string) error {
	file, err := s.repo.GetFile(ctx, id)
	if err != nil {
		return fmt.Errorf("getting file: %w", err)
	}
	if file == nil {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}

	if err := s.blobs.DeleteFile(ctx, file.DriveFileID); err != nil {
		return fmt.Errorf("deleting from blob storage: %w", err)
	}

	if err := s.repo.DeleteFile(ctx, id); err != nil {
		return fmt.Errorf("deleting metadata: %w", err)
	}

	return nil
}

// Sync scans blob storage and indexes all files into the database.
func (s *Service) Sync(ctx context.Context) (int, error) {
	var allFiles []model.File
	pageToken := ""

	for {
		objects, nextToken, err := s.blobs.ListFiles(ctx, "", pageToken, 100)
		if err != nil {
			return 0, fmt.Errorf("listing blob files: %w", err)
		}

		now := time.Now()
		for _, obj := range objects {
			createdAt, _ := time.Parse(time.RFC3339, obj.CreatedTime)
			if createdAt.IsZero() {
				createdAt = now
			}

			file := model.File{
				ID:           obj.ID,
				DriveFileID:  obj.ID,
				Name:         obj.Name,
				MimeType:     obj.MimeType,
				Size:         obj.Size,
				Category:     model.CategoryOther,
				Tags:         []string{},
				DrivePath:    buildDrivePath(obj.Parents),
				ThumbnailURL: obj.ThumbnailURL,
				WebViewURL:   obj.WebViewURL,
				CreatedAt:    createdAt,
				UpdatedAt:    now,
			}
			allFiles = append(allFiles, file)
		}

		if nextToken == "" {
			break
		}
		pageToken = nextToken
	}

	if len(allFiles) > 0 {
		if err := s.repo.BatchPutFiles(ctx, allFiles); err != nil {
			return 0, fmt.Errorf("batch storing metadata: %w", err)
		}
	}

	return len(allFiles), nil
}

// ListCategories returns all categories with file counts.
func (s *Service) ListCategories(ctx context.Context) ([]model.CategoryCount, error) {
	categories, err := s.repo.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	return categories, nil
}

// ListTags returns all unique tags.
func (s *Service) ListTags(ctx context.Context) ([]string, error) {
	tags, err := s.repo.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	return tags, nil
}

func buildDrivePath(parents []string) string {
	if len(parents) > 0 {
		return parents[0]
	}
	return "root"
}
