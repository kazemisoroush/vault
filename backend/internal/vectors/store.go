// Package vectors stores file embeddings and finds the nearest ones to a query.
package vectors

import "context"

//go:generate go tool mockgen -source=store.go -destination=../mocks/vectorstore_mock.go -package=mocks -mock_names=Store=MockVectorStore

// Store keeps one vector per file, keyed by file id, and returns the nearest ids to a query.
type Store interface {
	Put(ctx context.Context, id string, vector []float32) error
	Query(ctx context.Context, vector []float32, topK int32) ([]string, error)
	Delete(ctx context.Context, id string) error
}
