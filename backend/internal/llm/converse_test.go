package llm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMessenger returns a canned reply per call, in order, and records the params it saw.
type fakeMessenger struct {
	replies []*anthropic.Message
	errs    []error
	calls   []anthropic.MessageNewParams
}

func (f *fakeMessenger) New(_ context.Context, body anthropic.MessageNewParams, _ ...option.RequestOption) (*anthropic.Message, error) {
	i := len(f.calls)
	f.calls = append(f.calls, body)
	if i < len(f.errs) && f.errs[i] != nil {
		return nil, f.errs[i]
	}
	return f.replies[i], nil
}

// fakeRecorder keeps the calls it was handed, so a test can assert the trace.
type fakeRecorder struct{ calls []Call }

func (r *fakeRecorder) Record(_ context.Context, call Call) { r.calls = append(r.calls, call) }

// textReply is a model reply that ends the turn with a plain text answer.
func textReply(text string) *anthropic.Message {
	return &anthropic.Message{
		StopReason: anthropic.StopReasonEndTurn,
		Content:    []anthropic.ContentBlockUnion{{Type: "text", Text: text}},
		Usage:      anthropic.Usage{InputTokens: 3, OutputTokens: 4},
	}
}

// toolReply is a model reply that asks to call one tool.
func toolReply(id, name, input string) *anthropic.Message {
	return &anthropic.Message{
		StopReason: anthropic.StopReasonToolUse,
		Content:    []anthropic.ContentBlockUnion{{Type: "tool_use", ID: id, Name: name, Input: json.RawMessage(input)}},
		Usage:      anthropic.Usage{InputTokens: 5, OutputTokens: 6},
	}
}

func TestConverseWithNoToolUseReturnsTheText(t *testing.T) {
	// Arrange
	client := &fakeMessenger{replies: []*anthropic.Message{textReply("the answer")}}
	recorder := &fakeRecorder{}
	model := newModel(client, "test-model", "agent", recorder)

	// Act
	got, err := model.Converse(context.Background(), Conversation{Prompt: "what is it", MaxTokens: 100})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "the answer", got)
	assert.Len(t, client.calls, 1)
	require.Len(t, recorder.calls, 1)
	assert.Equal(t, "what is it", recorder.calls[0].Prompt)
}

func TestConverseFeedsToolResultBackThenReturnsFinalText(t *testing.T) {
	// Arrange: the model calls a tool, then answers on the next turn.
	client := &fakeMessenger{replies: []*anthropic.Message{
		toolReply("t1", "search", `{"q":"fuel"}`),
		textReply("found it"),
	}}
	recorder := &fakeRecorder{}
	model := newModel(client, "test-model", "agent", recorder)

	var seen ToolCall
	execute := func(_ context.Context, call ToolCall) (string, error) {
		seen = call
		return "one result", nil
	}

	// Act
	got, err := model.Converse(context.Background(), Conversation{
		Prompt:    "find fuel",
		MaxTokens: 100,
		Tools:     []Tool{{Name: "search", Description: "search by meaning", Schema: map[string]any{"q": map[string]any{"type": "string"}}, Required: []string{"q"}}},
		Execute:   execute,
	})

	// Assert: the tool ran with the model's input, the second turn carried the result, and both
	// model calls were recorded.
	require.NoError(t, err)
	assert.Equal(t, "found it", got)
	assert.Equal(t, "search", seen.Name)
	assert.JSONEq(t, `{"q":"fuel"}`, string(seen.Input))
	assert.Len(t, client.calls, 2)
	assert.Len(t, recorder.calls, 2)
	// The second call to the model carries three turns: the question, the assistant tool use,
	// and the tool result.
	assert.Len(t, client.calls[1].Messages, 3)
}

func TestConverseReportsAToolErrorBackToTheModel(t *testing.T) {
	// Arrange: the tool fails, the model still answers on the next turn.
	client := &fakeMessenger{replies: []*anthropic.Message{
		toolReply("t1", "search", `{"q":"x"}`),
		textReply("handled the failure"),
	}}
	model := newModel(client, "test-model", "agent", &fakeRecorder{})
	execute := func(_ context.Context, _ ToolCall) (string, error) {
		return "", errors.New("store down")
	}

	// Act
	got, err := model.Converse(context.Background(), Conversation{
		Prompt:    "find x",
		MaxTokens: 100,
		Tools:     []Tool{{Name: "search"}},
		Execute:   execute,
	})

	// Assert: no abort; the loop continued to a final answer.
	require.NoError(t, err)
	assert.Equal(t, "handled the failure", got)
	assert.Len(t, client.calls, 2)
}

func TestConverseStopsAtTheRoundCap(t *testing.T) {
	// Arrange: the model keeps calling a tool and never answers.
	client := &fakeMessenger{replies: []*anthropic.Message{
		toolReply("t1", "loop", `{}`),
		toolReply("t2", "loop", `{}`),
		toolReply("t3", "loop", `{}`),
	}}
	model := newModel(client, "test-model", "agent", &fakeRecorder{})
	execute := func(_ context.Context, _ ToolCall) (string, error) { return "again", nil }

	// Act: cap the exchange at two rounds.
	_, err := model.Converse(context.Background(), Conversation{
		Prompt:    "loop forever",
		MaxTokens: 100,
		Tools:     []Tool{{Name: "loop"}},
		Execute:   execute,
		MaxRounds: 2,
	})

	// Assert: it gives up after the cap rather than looping forever.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "within 2 rounds")
	assert.Len(t, client.calls, 2)
}

func TestCompleteStillReturnsTheReplyAndRecords(t *testing.T) {
	// Arrange: the existing Complete path must keep working after the refactor.
	client := &fakeMessenger{replies: []*anthropic.Message{textReply("plain reply")}}
	recorder := &fakeRecorder{}
	model := newModel(client, "test-model", "extract", recorder)

	// Act
	got, err := model.Complete(context.Background(), "a prompt", 100, anthropic.NewTextBlock("hello"))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "plain reply", got)
	require.Len(t, recorder.calls, 1)
	assert.True(t, recorder.calls[0].OK)
	assert.Equal(t, int64(3), recorder.calls[0].InputTokens)
}

func TestSendRecordsAFailedCall(t *testing.T) {
	// Arrange
	client := &fakeMessenger{replies: []*anthropic.Message{nil}, errs: []error{errors.New("bedrock 500")}}
	recorder := &fakeRecorder{}
	model := newModel(client, "test-model", "agent", recorder)

	// Act
	_, err := model.Complete(context.Background(), "a prompt", 100, anthropic.NewTextBlock("hi"))

	// Assert: the error is wrapped and the failed call is still on the trace.
	require.Error(t, err)
	require.Len(t, recorder.calls, 1)
	assert.False(t, recorder.calls[0].OK)
	assert.Contains(t, recorder.calls[0].Error, "bedrock 500")
}
