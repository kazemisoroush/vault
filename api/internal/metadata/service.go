package metadata

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kazemisoroush/vault/api/internal/drive"
	"github.com/kazemisoroush/vault/api/internal/model"
	"github.com/kazemisoroush/vault/api/internal/storage"
)

// Service coordinates Drive and storage operations.
type Service struct {
	drive drive.Service
	repo  storage.Repository
}

// NewService creates a new metadata service.
func NewService(drive drive.Service, repo storage.Repository) *Service {
	return &Service{drive: drive, repo: repo}
}

// ListFiles returns files matching the query.
func (s *Service) ListFiles(ctx context.Context, query model.FileQuery) (*model.FileListResult, error) {
	return s.repo.ListFiles(ctx, query)
}

// GetFile returns a single file's metadata.
func (s *Service) GetFile(ctx context.Context, id string) (*model.File, error) {
	return s.repo.GetFile(ctx, id)
}

// UploadFile uploads to Drive and stores metadata.
func (s *Service) UploadFile(ctx context.Context, req UploadRequest) (*model.File, error) {
	driveFile, err := s.drive.UploadFile(ctx, req.Name, req.MimeType, req.FolderID, req.Content)
	if err != nil {
		return nil, fmt.Errorf("uploading to drive: %w", err)
	}

	now := time.Now()
	file := model.File{
		ID:           uuid.New().String(),
		DriveFileID:  driveFile.ID,
		Name:         driveFile.Name,
		MimeType:     driveFile.MimeType,
		Size:         driveFile.Size,
		Category:     req.Category,
		Tags:         req.Tags,
		DrivePath:    buildDrivePath(driveFile.Parents),
		ThumbnailURL: driveFile.ThumbnailURL,
		WebViewURL:   driveFile.WebViewURL,
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
		return nil, fmt.Errorf("file not found: %s", id)
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

// DeleteFile removes from Drive and deletes metadata.
func (s *Service) DeleteFile(ctx context.Context, id string) error {
	file, err := s.repo.GetFile(ctx, id)
	if err != nil {
		return fmt.Errorf("getting file: %w", err)
	}
	if file == nil {
		return fmt.Errorf("file not found: %s", id)
	}

	if err := s.drive.DeleteFile(ctx, file.DriveFileID); err != nil {
		return fmt.Errorf("deleting from drive: %w", err)
	}

	if err := s.repo.DeleteFile(ctx, id); err != nil {
		return fmt.Errorf("deleting metadata: %w", err)
	}

	return nil
}

// Sync scans Google Drive and indexes all files into the database.
func (s *Service) Sync(ctx context.Context) (int, error) {
	var allFiles []model.File
	pageToken := ""

	for {
		driveFiles, nextToken, err := s.drive.ListFiles(ctx, "", pageToken, 100)
		if err != nil {
			return 0, fmt.Errorf("listing drive files: %w", err)
		}

		now := time.Now()
		for _, df := range driveFiles {
			createdAt, _ := time.Parse(time.RFC3339, df.CreatedTime)
			if createdAt.IsZero() {
				createdAt = now
			}

			file := model.File{
				ID:           uuid.New().String(),
				DriveFileID:  df.ID,
				Name:         df.Name,
				MimeType:     df.MimeType,
				Size:         df.Size,
				Category:     model.CategoryOther,
				Tags:         []string{},
				DrivePath:    buildDrivePath(df.Parents),
				ThumbnailURL: df.ThumbnailURL,
				WebViewURL:   df.WebViewURL,
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
	return s.repo.ListCategories(ctx)
}

// ListTags returns all unique tags.
func (s *Service) ListTags(ctx context.Context) ([]string, error) {
	return s.repo.ListTags(ctx)
}

func buildDrivePath(parents []string) string {
	if len(parents) > 0 {
		return parents[0]
	}
	return "root"
}
