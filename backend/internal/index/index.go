// Package index stores file records and makes them searchable.
package index

import (
	"context"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

//go:generate go tool mockgen -source=index.go -destination=../mocks/index_mock.go -package=mocks

// Index persists file records.
type Index interface {
	Put(ctx context.Context, file domain.File) error
	// Get returns a record by id without an ownership check, so a caller serving a user must verify it.
	Get(ctx context.Context, id string) (domain.File, error)
	// List returns one page of the owner's records.
	List(ctx context.Context, owner string, limit int32, cursor string) ([]domain.File, string, error)
	// ListAll returns one page of every record, for system callers such as the backfill.
	ListAll(ctx context.Context, limit int32, cursor string) ([]domain.File, string, error)
	// Delete removes a record by id without an ownership check.
	Delete(ctx context.Context, id string) error
}
