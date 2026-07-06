// Package index stores file records and makes them searchable.
package index

import (
	"context"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

//go:generate go tool mockgen -source=index.go -destination=../mocks/index_mock.go -package=mocks

// Index persists file records. List scopes to an owner; an empty owner lists every record, for
// system callers such as the backfill. Get and Delete are unscoped, so callers that serve a user
// must check ownership themselves.
type Index interface {
	Put(ctx context.Context, file domain.File) error
	Get(ctx context.Context, id string) (domain.File, error)
	List(ctx context.Context, owner string, limit int32, cursor string) ([]domain.File, string, error)
	Delete(ctx context.Context, id string) error
}
