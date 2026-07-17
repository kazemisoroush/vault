package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/kb"
	"github.com/kazemisoroush/vault/backend/internal/llm"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// scriptedModel is a fake Converser that runs a fixed list of tool calls, records their results,
// and then returns a fixed final reply, standing in for the real model.
type scriptedModel struct {
	calls   []llm.ToolCall
	final   string
	results []string
}

func (m *scriptedModel) Converse(ctx context.Context, c llm.Conversation) (string, error) {
	for _, call := range m.calls {
		out, err := c.Execute(ctx, call)
		if err != nil {
			return "", fmt.Errorf("execute %s: %w", call.Name, err)
		}
		m.results = append(m.results, out)
	}
	return m.final, nil
}

// fakeRetriever returns fixed passages, or an error, standing in for hybrid retrieval over the KB.
type fakeRetriever struct {
	passages []kb.Passage
	err      error
}

func (f fakeRetriever) Retrieve(_ context.Context, _ string, _ int) ([]kb.Passage, error) {
	return f.passages, f.err
}

func newAgent(t *testing.T, model Converser, r retriever) (*Agent, *mocks.MockIndex) {
	t.Helper()
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	return NewAgent(model, r, idx), idx
}

func TestAnswerRunsSearchAndReturnsCitedFiles(t *testing.T) {
	// Arrange: the model searches, then answers citing one file.
	model := &scriptedModel{
		calls: []llm.ToolCall{{Name: toolSearchByMeaning, Input: []byte(`{"query":"fuel","limit":20}`)}},
		final: `{"answer":"your last fill was at Shell","fileIds":["a"]}`,
	}
	r := fakeRetriever{passages: []kb.Passage{
		{FileID: "a", FileName: "petrol", Text: "Shell fuel receipt"},
		{FileID: "b", FileName: "ticket", Text: "a flight ticket"},
	}}
	a, idx := newAgent(t, model, r)
	idx.EXPECT().Get(gomock.Any(), "a").Return(domain.File{ID: "a", OwnerID: "alice", Name: "petrol", Key: "files/a"}, nil).AnyTimes()
	idx.EXPECT().Get(gomock.Any(), "b").Return(domain.File{ID: "b", OwnerID: "alice", Name: "ticket", Key: "files/b"}, nil).AnyTimes()

	// Act
	result, err := a.Answer(context.Background(), "alice", "where did I last buy fuel")

	// Assert: the answer and the one cited file come back, and the search returned both passages
	// because both belong to the caller.
	require.NoError(t, err)
	assert.Equal(t, "your last fill was at Shell", result.Text)
	require.Len(t, result.Files, 1)
	assert.Equal(t, "a", result.Files[0].ID)
	assert.Contains(t, model.results[0], `"fileId":"a"`)
	assert.Contains(t, model.results[0], `"fileId":"b"`)
}

func TestSearchDropsAForeignOwnerPassage(t *testing.T) {
	// Arrange: retrieval returns two passages, but only one file belongs to the caller. The
	// managed Knowledge Base is shared, so the foreign passage must be scoped out before the
	// model sees it.
	model := &scriptedModel{
		calls: []llm.ToolCall{{Name: toolSearchByMeaning, Input: []byte(`{"query":"deposit"}`)}},
		final: `{"answer":"done","fileIds":[]}`,
	}
	r := fakeRetriever{passages: []kb.Passage{
		{FileID: "mine", FileName: "mine", Text: "my own deposit note"},
		{FileID: "theirs", FileName: "theirs", Text: "another owner's deposit note"},
	}}
	a, idx := newAgent(t, model, r)
	idx.EXPECT().Get(gomock.Any(), "mine").Return(domain.File{ID: "mine", OwnerID: "alice"}, nil).AnyTimes()
	idx.EXPECT().Get(gomock.Any(), "theirs").Return(domain.File{ID: "theirs", OwnerID: "mallory"}, nil).AnyTimes()

	// Act
	_, err := a.Answer(context.Background(), "alice", "show my deposit note")

	// Assert: only the caller's passage reaches the model.
	require.NoError(t, err)
	assert.Contains(t, model.results[0], `"fileId":"mine"`)
	assert.NotContains(t, model.results[0], `"fileId":"theirs"`)
	assert.NotContains(t, model.results[0], "another owner")
}

func TestAnswerGetFileHidesAForeignOwner(t *testing.T) {
	// Arrange: the model asks for a file that belongs to someone else.
	model := &scriptedModel{
		calls: []llm.ToolCall{{Name: toolGetFile, Input: []byte(`{"id":"secret"}`)}},
		final: `{"answer":"I could not find that","fileIds":[]}`,
	}
	a, idx := newAgent(t, model, fakeRetriever{})
	idx.EXPECT().Get(gomock.Any(), "secret").Return(domain.File{ID: "secret", OwnerID: "mallory"}, nil)

	// Act
	result, err := a.Answer(context.Background(), "alice", "open secret")

	// Assert: the tool reports not found, so the foreign file never leaks.
	require.NoError(t, err)
	assert.Equal(t, `{"error":"file not found"}`, model.results[0])
	assert.Empty(t, result.Files)
}

func TestAnswerDropsACitedFileTheCallerDoesNotOwn(t *testing.T) {
	// Arrange: the model cites an id whose record belongs to another owner.
	model := &scriptedModel{final: `{"answer":"here","fileIds":["foreign"]}`}
	a, idx := newAgent(t, model, fakeRetriever{})
	idx.EXPECT().Get(gomock.Any(), "foreign").Return(domain.File{ID: "foreign", OwnerID: "bob"}, nil)

	// Act
	result, err := a.Answer(context.Background(), "alice", "anything")

	// Assert: the cited file is dropped because alice does not own it.
	require.NoError(t, err)
	assert.Equal(t, "here", result.Text)
	assert.Empty(t, result.Files)
}

func TestAnswerPropagatesAToolError(t *testing.T) {
	// Arrange: retrieval fails, which the executor returns as an error.
	model := &scriptedModel{calls: []llm.ToolCall{{Name: toolSearchByMeaning, Input: []byte(`{"query":"x"}`)}}}
	a, _ := newAgent(t, model, fakeRetriever{err: errors.New("bedrock down")})

	// Act
	_, err := a.Answer(context.Background(), "alice", "x")

	// Assert
	require.Error(t, err)
}

func TestParseFinalFallsBackToPlainText(t *testing.T) {
	// Arrange + Act: a reply that is not the expected object.
	answer, ids := parseFinal("I could not format that")

	// Assert: the whole reply becomes the answer with no cited ids.
	assert.Equal(t, "I could not format that", answer)
	assert.Nil(t, ids)
}
