// Package vectors stores file embeddings and finds the nearest ones to a query.
package vectors

import "context"

//go:generate go tool mockgen -source=store.go -destination=../mocks/vectorstore_mock.go -package=mocks -mock_names=Store=MockVectorStore

// Store keeps a file's chunk vectors, all tagged with its owner, and returns the nearest distinct
// file ids owned by the caller. A file is embedded as several chunks (name, each metadata field,
// body passages), so Put takes all of a file's vectors and Query dedupes chunk hits back to files.
type Store interface {
	Put(ctx context.Context, fileID string, ownerID string, vectors [][]float32) error
	Query(ctx context.Context, ownerID string, vector []float32, topK int32) ([]string, error)
	Delete(ctx context.Context, fileID string) error
}
