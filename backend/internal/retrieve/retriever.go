// Package retrieve answers a natural-language query over the matched files.
package retrieve

import (
	"context"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

//go:generate go tool mockgen -source=retriever.go -destination=../mocks/retriever_mock.go -package=mocks

// Retriever answers a query over the given files: the matching ids and a human-readable answer.
type Retriever interface {
	Match(ctx context.Context, query string, files []domain.File) (Answer, error)
}
