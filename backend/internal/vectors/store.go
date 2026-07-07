// Package vectors stores file embeddings and finds the nearest ones to a query.
package vectors

import "context"

//go:generate go tool mockgen -source=store.go -destination=../mocks/vectorstore_mock.go -package=mocks -mock_names=Store=MockVectorStore

// Store keeps one owner-tagged vector per file and returns the nearest ids owned by the caller.
type Store interface {
	Put(ctx context.Context, id string, ownerID string, vector []float32) error
	Query(ctx context.Context, ownerID string, vector []float32, topK int32) ([]string, error)
	Delete(ctx context.Context, id string) error
}
