// Package extract turns a file's content into free-form metadata and canonical text via an LLM.
package extract

import (
	"context"
	"errors"
)

//go:generate go tool mockgen -source=extractor.go -destination=../mocks/extractor_mock.go -package=mocks

// ErrRetryable marks an extraction that failed for a transient reason, such as the model being
// throttled. A caller can redrive the work later rather than treat the file as failed. It is the
// extract seam's own signal, so callers need not know how the underlying model reports throttling.
var ErrRetryable = errors.New("extraction temporarily unavailable")

// Extraction is what one file yields: a flat searchable metadata map, and the file's canonical
// text. Text is what a person would read in the file, in reading order. For text-bearing formats
// (plain text, office documents) it is decoded deterministically; for images and PDFs it is the
// model's transcription, captured once at ingest and stored as the record that later checks
// verify quoted spans against. Empty when the file has no readable text.
type Extraction struct {
	Meta map[string]string
	Text string
}

// Extractor reads a file's bytes and returns its extraction. A transient failure is wrapped
// with ErrRetryable so the caller can tell it apart from a genuinely unreadable file.
type Extractor interface {
	Extract(ctx context.Context, content []byte, contentType string) (Extraction, error)
}
