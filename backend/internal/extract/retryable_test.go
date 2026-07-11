package extract

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

func TestWrapExtractErrorTagsATransientFailure(t *testing.T) {
	// Arrange: the model reported a throttle it could not shake off.
	err := wrapExtractError(llm.NewRetryableError(errors.New("429 throttled")))

	// Assert: it surfaces on the extract seam as ErrRetryable, so ingest can redrive it.
	assert.ErrorIs(t, err, ErrRetryable)
}

func TestWrapExtractErrorLeavesATerminalFailure(t *testing.T) {
	// Arrange: a plain failure, for example an unreadable file.
	err := wrapExtractError(errors.New("bad request"))

	// Assert: it is not tagged retryable, so the file fails fast as before.
	assert.False(t, errors.Is(err, ErrRetryable))
}
