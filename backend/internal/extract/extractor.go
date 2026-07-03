// Package extract turns a file's content into free-form metadata via an LLM.
package extract

import "context"

//go:generate go tool mockgen -source=extractor.go -destination=../mocks/extractor_mock.go -package=mocks

// Extractor reads a file's bytes and returns a flat metadata map.
type Extractor interface {
	Extract(ctx context.Context, content []byte, contentType string) (map[string]string, error)
}
