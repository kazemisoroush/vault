package storage

import (
	"context"

	"github.com/kazemisoroush/vault/api/internal/model"
)

//go:generate go tool mockgen -source=repository.go -destination=repository_mock.go -package=storage

// Repository defines the interface for metadata persistence.
type Repository interface {
	PutFile(ctx context.Context, file model.File) error
	GetFile(ctx context.Context, id string) (*model.File, error)
	DeleteFile(ctx context.Context, id string) error
	ListFiles(ctx context.Context, query model.FileQuery) (*model.FileListResult, error)
	ListCategories(ctx context.Context) ([]model.CategoryCount, error)
	ListTags(ctx context.Context) ([]string, error)
	BatchPutFiles(ctx context.Context, files []model.File) error
}
