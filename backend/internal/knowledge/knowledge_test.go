package knowledge_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/knowledge"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// fileWith builds a ready file owned by alice with the given id and attributes.
func fileWith(id string, attrs domain.Attributes, created time.Time) domain.File {
	return domain.File{ID: id, OwnerID: "alice", Name: id + ".pdf", Attributes: attrs, CreatedAt: created}
}

func TestSearchReturnsNearestOwnedFilesPassingTheFilter(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	embedder := mocks.NewMockEmbedder(ctrl)
	store := mocks.NewMockVectorStore(ctrl)
	idx := mocks.NewMockIndex(ctrl)

	vector := []float32{0.1, 0.2}
	receipt := fileWith("a", domain.Attributes{DocType: "receipt", Vendor: "Shell"}, time.Now())
	ticket := fileWith("b", domain.Attributes{DocType: "ticket"}, time.Now())

	embedder.EXPECT().Embed(gomock.Any(), "fuel").Return(vector, nil)
	store.EXPECT().Query(gomock.Any(), "alice", vector, int32(20)).Return([]string{"a", "b"}, nil)
	idx.EXPECT().Get(gomock.Any(), "a").Return(receipt, nil)
	idx.EXPECT().Get(gomock.Any(), "b").Return(ticket, nil)

	base := knowledge.NewStore(embedder, store, idx)

	// Act: keep only receipts.
	got, err := base.Search(context.Background(), "alice", "fuel", knowledge.Filter{DocType: "receipt"}, 20)

	// Assert: the ticket is dropped, the receipt stays, order preserved.
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "a", got[0].ID)
}

func TestSearchSkipsFilesOwnedByAnotherCaller(t *testing.T) {
	// Arrange: the vector store returns an id whose record belongs to someone else.
	ctrl := gomock.NewController(t)
	embedder := mocks.NewMockEmbedder(ctrl)
	store := mocks.NewMockVectorStore(ctrl)
	idx := mocks.NewMockIndex(ctrl)

	vector := []float32{0.3}
	foreign := domain.File{ID: "x", OwnerID: "mallory", Name: "x.pdf"}

	embedder.EXPECT().Embed(gomock.Any(), "passport").Return(vector, nil)
	store.EXPECT().Query(gomock.Any(), "alice", vector, int32(5)).Return([]string{"x"}, nil)
	idx.EXPECT().Get(gomock.Any(), "x").Return(foreign, nil)

	base := knowledge.NewStore(embedder, store, idx)

	// Act
	got, err := base.Search(context.Background(), "alice", "passport", knowledge.Filter{}, 5)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestQueryFiltersOwnerFilesByAttributesAndTime(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	embedder := mocks.NewMockEmbedder(ctrl)
	store := mocks.NewMockVectorStore(ctrl)
	idx := mocks.NewMockIndex(ctrl)

	jan := time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
	old := fileWith("old", domain.Attributes{Person: "Sara Jabbari"}, jan)
	recent := fileWith("recent", domain.Attributes{Person: "Sara Jabbari"}, mar)
	other := fileWith("other", domain.Attributes{Person: "Soroush"}, mar)

	idx.EXPECT().List(gomock.Any(), "alice", int32(100), "").Return([]domain.File{old, recent, other}, "", nil)

	base := knowledge.NewStore(embedder, store, idx)

	// Act: Sara's files from February onward.
	got, err := base.Query(context.Background(), "alice", knowledge.Filter{
		Person: "sara",
		Since:  time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
	})

	// Assert: only the recent Sara file survives both filters.
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "recent", got[0].ID)
}

func TestQueryPagesThroughEveryOwnerRecord(t *testing.T) {
	// Arrange: the index returns two pages before the cursor empties.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)

	first := fileWith("one", domain.Attributes{}, time.Now())
	second := fileWith("two", domain.Attributes{}, time.Now())
	idx.EXPECT().List(gomock.Any(), "alice", int32(100), "").Return([]domain.File{first}, "next", nil)
	idx.EXPECT().List(gomock.Any(), "alice", int32(100), "next").Return([]domain.File{second}, "", nil)

	base := knowledge.NewStore(nil, nil, idx)

	// Act
	got, err := base.Query(context.Background(), "alice", knowledge.Filter{})

	// Assert: both pages are collected.
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestFetchReturnsOwnedFileWithItsText(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)

	file := domain.File{ID: "a", OwnerID: "alice", Name: "petrol.txt", Meta: map[string]string{"vendor": "Shell"}}
	idx.EXPECT().Get(gomock.Any(), "a").Return(file, nil)

	base := knowledge.NewStore(nil, nil, idx)

	// Act
	doc, err := base.Fetch(context.Background(), "alice", "a")

	// Assert: the document carries the file and its search text.
	require.NoError(t, err)
	assert.Equal(t, "a", doc.File.ID)
	assert.Equal(t, "petrol.txt\nvendor: Shell", doc.Text)
}

func TestFetchHidesAFileOwnedByAnotherCaller(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)

	file := domain.File{ID: "a", OwnerID: "mallory", Name: "secret.pdf"}
	idx.EXPECT().Get(gomock.Any(), "a").Return(file, nil)

	base := knowledge.NewStore(nil, nil, idx)

	// Act
	_, err := base.Fetch(context.Background(), "alice", "a")

	// Assert: it looks the same as a missing file, so existence never leaks.
	require.ErrorIs(t, err, index.ErrNotFound)
}

func TestFetchPropagatesAMissingFile(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)

	idx.EXPECT().Get(gomock.Any(), "gone").Return(domain.File{}, index.ErrNotFound)

	base := knowledge.NewStore(nil, nil, idx)

	// Act
	_, err := base.Fetch(context.Background(), "alice", "gone")

	// Assert
	require.ErrorIs(t, err, index.ErrNotFound)
}

func TestSearchWrapsAnEmbedderError(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	embedder := mocks.NewMockEmbedder(ctrl)

	embedder.EXPECT().Embed(gomock.Any(), "x").Return(nil, errors.New("bedrock down"))

	base := knowledge.NewStore(embedder, nil, nil)

	// Act
	_, err := base.Search(context.Background(), "alice", "x", knowledge.Filter{}, 3)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embed query")
}
