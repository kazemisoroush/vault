// Package retrieve finds the files that match a natural-language query.
package retrieve

import (
	"context"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

//go:generate go tool mockgen -source=retriever.go -destination=../mocks/retriever_mock.go -package=mocks

// Retriever returns the ids of the files matching a query, most relevant first.
type Retriever interface {
	Match(ctx context.Context, query string, files []domain.File) ([]string, error)
}
