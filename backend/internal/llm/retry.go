package llm

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// Retry defaults bound how hard a throttled or failing model call is retried before it is handed
// back to the caller. Kept well within the Lambda timeout so an extraction has room to finish.
const (
	defaultMaxAttempts = 4
	defaultBaseDelay   = 500 * time.Millisecond
	defaultMaxDelay    = 8 * time.Second
)

// RetryableError marks a model failure that is transient, such as throttling, so a caller can
// redrive the work later rather than treat it as terminal.
type RetryableError struct{ err error }

// NewRetryableError wraps err as retryable. send uses it; callers may also use it in tests.
func NewRetryableError(err error) *RetryableError { return &RetryableError{err: err} }

// Error returns the underlying message.
func (e *RetryableError) Error() string { return e.err.Error() }

// Unwrap exposes the underlying error for errors.Is and errors.As.
func (e *RetryableError) Unwrap() error { return e.err }

// retryable reports whether an error is worth trying again: throttling (429), a request timeout
// (408), or a server-side 5xx. A client error such as 400 is terminal and is not retried.
func retryable(err error) bool {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	code := apiErr.StatusCode
	return code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || code >= 500
}

// backoff returns a randomised delay for a retry attempt. It grows exponentially up to a cap and
// draws uniformly below that (full jitter), so a burst of retries does not resynchronise into a
// fresh spike against the model.
func backoff(attempt int, base, max time.Duration) time.Duration {
	span := base << attempt
	if span <= 0 || span > max {
		span = max
	}
	return time.Duration(rand.Int63n(int64(span) + 1))
}

// sleepFor waits for d, or returns early if the context is cancelled.
func sleepFor(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("backoff wait: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
