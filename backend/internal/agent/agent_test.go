package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/domain"
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

func newAgent(t *testing.T, model Converser) (*Agent, *mocks.MockEmbedder, *mocks.MockVectorStore, *mocks.MockIndex) {
	t.Helper()
	ctrl := gomock.NewController(t)
	embedder := mocks.NewMockEmbedder(ctrl)
	store := mocks.NewMockVectorStore(ctrl)
	idx := mocks.NewMockIndex(ctrl)
	return NewAgent(model, embedder, store, idx), embedder, store, idx
}

func TestAnswerRunsSearchAndReturnsCitedFiles(t *testing.T) {
	// Arrange: the model searches by meaning, then answers citing one file.
	model := &scriptedModel{
		calls: []llm.ToolCall{{Name: toolSearch, Input: []byte(`{"query":"fuel","limit":20}`)}},
		final: `{"answer":"your last fill was at Shell","ids":["a"]}`,
	}
	a, embedder, store, idx := newAgent(t, model)
	vec := []float32{0.1, 0.2}
	fileA := domain.File{ID: "a", OwnerID: "alice", Name: "petrol", Key: "files/a"}
	fileB := domain.File{ID: "b", OwnerID: "alice", Name: "ticket", Key: "files/b"}

	embedder.EXPECT().Embed(gomock.Any(), "fuel").Return(vec, nil)
	store.EXPECT().Query(gomock.Any(), "alice", vec, int32(20)).Return([]string{"a", "b"}, nil)
	idx.EXPECT().Get(gomock.Any(), "a").Return(fileA, nil).AnyTimes()
	idx.EXPECT().Get(gomock.Any(), "b").Return(fileB, nil).AnyTimes()

	// Act
	result, err := a.Answer(context.Background(), "alice", "where did I last buy fuel")

	// Assert: the answer and the one cited file come back, and the tool saw both nearest files.
	require.NoError(t, err)
	assert.Equal(t, "your last fill was at Shell", result.Text)
	require.Len(t, result.Files, 1)
	assert.Equal(t, "a", result.Files[0].ID)
	assert.Contains(t, model.results[0], `"id":"a"`)
	assert.Contains(t, model.results[0], `"id":"b"`)
}

func TestAnswerFindByFactsFiltersByField(t *testing.T) {
	// Arrange: the model filters by a metadata value.
	model := &scriptedModel{
		calls: []llm.ToolCall{{Name: toolFacts, Input: []byte(`{"contains":[{"field":"vendor","value":"shell"}]}`)}},
		final: `{"answer":"","ids":["shell"]}`,
	}
	a, _, _, idx := newAgent(t, model)
	shell := domain.File{ID: "shell", OwnerID: "alice", Name: "s", Key: "files/shell", Meta: map[string]string{"vendor": "Shell"}}
	coles := domain.File{ID: "coles", OwnerID: "alice", Name: "c", Key: "files/coles", Meta: map[string]string{"vendor": "Coles"}}

	idx.EXPECT().List(gomock.Any(), "alice", listPageSize, "").Return([]domain.File{shell, coles}, "", nil)
	idx.EXPECT().Get(gomock.Any(), "shell").Return(shell, nil)

	// Act
	result, err := a.Answer(context.Background(), "alice", "shell receipts")

	// Assert: only the Shell file passed the filter and it is the cited file.
	require.NoError(t, err)
	assert.Contains(t, model.results[0], `"id":"shell"`)
	assert.NotContains(t, model.results[0], `"id":"coles"`)
	require.Len(t, result.Files, 1)
	assert.Equal(t, "shell", result.Files[0].ID)
}

func TestAnswerGetFileHidesAForeignOwner(t *testing.T) {
	// Arrange: the model asks for a file that belongs to someone else.
	model := &scriptedModel{
		calls: []llm.ToolCall{{Name: toolGet, Input: []byte(`{"id":"secret"}`)}},
		final: `{"answer":"I could not find that","ids":[]}`,
	}
	a, _, _, idx := newAgent(t, model)
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
	model := &scriptedModel{final: `{"answer":"here","ids":["foreign"]}`}
	a, _, _, idx := newAgent(t, model)
	idx.EXPECT().Get(gomock.Any(), "foreign").Return(domain.File{ID: "foreign", OwnerID: "bob"}, nil)

	// Act
	result, err := a.Answer(context.Background(), "alice", "anything")

	// Assert: the cited file is dropped because alice does not own it.
	require.NoError(t, err)
	assert.Equal(t, "here", result.Text)
	assert.Empty(t, result.Files)
}

func TestAnswerPropagatesAToolError(t *testing.T) {
	// Arrange: the search embedding fails, which the executor returns as an error.
	model := &scriptedModel{calls: []llm.ToolCall{{Name: toolSearch, Input: []byte(`{"query":"x"}`)}}}
	a, embedder, _, _ := newAgent(t, model)
	embedder.EXPECT().Embed(gomock.Any(), "x").Return(nil, errors.New("bedrock down"))

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

func TestFindByFactsRespectsTheTimeBound(t *testing.T) {
	// Arrange: two files, one before and one after the since date.
	jan := time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
	model := &scriptedModel{
		calls: []llm.ToolCall{{Name: toolFacts, Input: []byte(`{"since":"2026-02-01T00:00:00Z"}`)}},
		final: `{"answer":"","ids":[]}`,
	}
	a, _, _, idx := newAgent(t, model)
	old := domain.File{ID: "old", OwnerID: "alice", CreatedAt: jan}
	recent := domain.File{ID: "recent", OwnerID: "alice", CreatedAt: mar}
	idx.EXPECT().List(gomock.Any(), "alice", listPageSize, "").Return([]domain.File{old, recent}, "", nil)

	// Act
	_, err := a.Answer(context.Background(), "alice", "recent files")

	// Assert: only the file created after the since date is returned by the tool.
	require.NoError(t, err)
	assert.Contains(t, model.results[0], `"id":"recent"`)
	assert.NotContains(t, model.results[0], `"id":"old"`)
}
