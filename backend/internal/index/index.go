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
	List(ctx context.Context, ownerID string, limit int32, cursor string) ([]domain.File, string, error)
	// ListByStatus returns up to limit records in the given lifecycle status, across all owners, so
	// the Knowledge Base syncer can find the landed files to advance to ingested.
	ListByStatus(ctx context.Context, status string, limit int32) ([]domain.File, error)
	// Delete removes a record by id without an ownership check.
	Delete(ctx context.Context, id string) error
}
