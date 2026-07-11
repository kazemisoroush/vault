// Package extract turns a file's content into free-form metadata via an LLM.
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

// Extractor reads a file's bytes and returns a flat metadata map. A transient failure is wrapped
// with ErrRetryable so the caller can tell it apart from a genuinely unreadable file.
type Extractor interface {
	Extract(ctx context.Context, content []byte, contentType string) (map[string]string, error)
}
