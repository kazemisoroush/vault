// Package vision turns files the Knowledge Base cannot parse on its own, images and PDFs, into
// searchable text with a vision model, so scanned IDs, passports, and photos become findable.
package vision

import "context"

//go:generate go tool mockgen -source=vision.go -destination=../mocks/vision_mock.go -package=mocks

// Transcriber turns an image or PDF into searchable text. *ClaudeTranscriber satisfies it; the
// interface lets the ingest handler be tested without a model.
type Transcriber interface {
	Transcribe(ctx context.Context, content []byte, contentType string) (string, error)
}
