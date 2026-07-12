package llm

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// instant makes retries take no wall-clock time in tests.
func instant(_ context.Context, _ time.Duration) error { return nil }

// apiError builds a Bedrock API error with the given status, populated enough that its Error()
// method can render (the SDK reads the request and response to format the message).
func apiError(status int) error {
	req, _ := http.NewRequest(http.MethodPost, "https://bedrock-runtime.us-east-1.amazonaws.com/v1/messages", nil)
	return &anthropic.Error{
		StatusCode: status,
		Request:    req,
		Response:   &http.Response{StatusCode: status},
	}
}

// throttled is a Bedrock 429, the error a burst of uploads trips.
func throttled() error { return apiError(http.StatusTooManyRequests) }

func TestSendRetriesThrottleThenSucceeds(t *testing.T) {
	// Arrange: two throttles, then a good reply.
	client := &fakeMessenger{
		replies: []*anthropic.Message{nil, nil, textReply("ok")},
		errs:    []error{throttled(), throttled(), nil},
	}
	recorder := &fakeRecorder{}
	model := newModel(client, "test-model", "extract", recorder)
	model.sleep = instant

	// Act
	got, err := model.Converse(context.Background(), Conversation{Prompt: "q", MaxTokens: 10})

	// Assert: it retried past the throttling, and every attempt is on the trace.
	require.NoError(t, err)
	assert.Equal(t, "ok", got)
	assert.Len(t, client.calls, 3)
	assert.Len(t, recorder.calls, 3)
}

func TestSendReturnsRetryableAfterExhausting(t *testing.T) {
	// Arrange: throttled on every attempt.
	client := &fakeMessenger{
		replies: []*anthropic.Message{nil, nil, nil, nil},
		errs:    []error{throttled(), throttled(), throttled(), throttled()},
	}
	model := newModel(client, "test-model", "extract", &fakeRecorder{})
	model.sleep = instant

	// Act
	_, err := model.Converse(context.Background(), Conversation{Prompt: "q", MaxTokens: 10})

	// Assert: it stops at the attempt cap and flags the failure as retryable for the caller.
	var retry *RetryableError
	require.ErrorAs(t, err, &retry)
	assert.Len(t, client.calls, defaultMaxAttempts)
}

func TestSendDoesNotRetryTerminalError(t *testing.T) {
	// Arrange: a 400 is a bad request, not worth retrying.
	client := &fakeMessenger{
		replies: []*anthropic.Message{nil},
		errs:    []error{apiError(http.StatusBadRequest)},
	}
	model := newModel(client, "test-model", "extract", &fakeRecorder{})
	model.sleep = instant

	// Act
	_, err := model.Converse(context.Background(), Conversation{Prompt: "q", MaxTokens: 10})

	// Assert: one attempt only, and it is not marked retryable.
	require.Error(t, err)
	var retry *RetryableError
	assert.False(t, errors.As(err, &retry))
	assert.Len(t, client.calls, 1)
}

func TestSendStopsRetryingWhenContextCancelled(t *testing.T) {
	// Arrange: throttled, but the context is cancelled during the first backoff.
	ctx, cancel := context.WithCancel(context.Background())
	client := &fakeMessenger{
		replies: []*anthropic.Message{nil, nil, nil, nil},
		errs:    []error{throttled(), throttled(), throttled(), throttled()},
	}
	model := newModel(client, "test-model", "extract", &fakeRecorder{})
	model.sleep = func(_ context.Context, _ time.Duration) error {
		cancel()
		return context.Canceled
	}

	// Act
	_, err := model.Converse(ctx, Conversation{Prompt: "q", MaxTokens: 10})

	// Assert: it gives up as soon as the context is done, after the first attempt.
	require.Error(t, err)
	assert.Len(t, client.calls, 1)
}
