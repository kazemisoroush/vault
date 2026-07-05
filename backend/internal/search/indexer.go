// Package search keeps the vector index in step with file records.
package search

import (
	"context"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

//go:generate go tool mockgen -source=indexer.go -destination=../mocks/indexer_mock.go -package=mocks

// Indexer keeps a file's search vector in step with its record.
type Indexer interface {
	Index(ctx context.Context, file domain.File) error
	Remove(ctx context.Context, id string) error
}
