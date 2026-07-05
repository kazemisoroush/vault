// Package embed turns text into a vector so files can be found by meaning.
package embed

import "context"

//go:generate go tool mockgen -source=embedder.go -destination=../mocks/embedder_mock.go -package=mocks

// Embedder turns text into a fixed-length vector.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
