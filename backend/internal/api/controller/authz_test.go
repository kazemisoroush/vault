package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
	"github.com/kazemisoroush/vault/backend/internal/retrieve"
)

// withOwner returns the request carrying an authenticated owner, as the auth middleware would.
func withOwner(r *http.Request, owner string) *http.Request {
	return r.WithContext(auth.WithOwner(r.Context(), owner))
}

func TestDropStampsTheOwner(t *testing.T) {
	// Arrange
	idx, blobs, store := mockDeps(t)
	c := NewFileController(idx, blobs, store)
	c.now = func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) }
	c.newID = func() string { return "test-id" }
	blobs.EXPECT().PresignPut(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("https://upload", nil)
	var saved domain.File
	idx.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, f domain.File) error {
		saved = f
		return nil
	})
	req := withOwner(httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(`{"name":"a.jpg","contentType":"image/jpeg","size":1}`)), "alice")

	// Act
	c.Drop(httptest.NewRecorder(), req)

	// Assert
	assert.Equal(t, "alice", saved.Owner)
}

func TestGetHidesAnotherOwnersFile(t *testing.T) {
	// Arrange: the record exists but belongs to bob; alice must see a 404, not a 403.
	idx, blobs, store := mockDeps(t)
	c := NewFileController(idx, blobs, store)
	idx.EXPECT().Get(gomock.Any(), "f1").Return(domain.File{ID: "f1", Owner: "bob", Key: "files/f1"}, nil)
	req := withOwner(httptest.NewRequest(http.MethodGet, "/files/f1", nil), "alice")
	req.SetPathValue("id", "f1")
	rec := httptest.NewRecorder()

	// Act
	c.Get(rec, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListScopesToTheCaller(t *testing.T) {
	// Arrange
	idx, blobs, store := mockDeps(t)
	c := NewFileController(idx, blobs, store)
	idx.EXPECT().List(gomock.Any(), "alice", gomock.Any(), gomock.Any()).Return(nil, "", nil)
	req := withOwner(httptest.NewRequest(http.MethodGet, "/files", nil), "alice")

	// Act
	c.List(httptest.NewRecorder(), req)

	// Assert: passing the owner to the index is the whole point; a nil return is fine.
}

func TestAskExcludesOtherOwnersFiles(t *testing.T) {
	// Arrange: the vector store returns files from two owners; only alice's may reach the model.
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	store := mocks.NewMockVectorStore(ctrl)
	embedder := mocks.NewMockEmbedder(ctrl)
	retriever := mocks.NewMockRetriever(ctrl)
	c := NewAskController(idx, blobs, embedder, store, retriever)

	embedder.EXPECT().Embed(gomock.Any(), "invoice").Return([]float32{0.1}, nil)
	store.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Return([]string{"mine", "theirs"}, nil)
	idx.EXPECT().Get(gomock.Any(), "mine").Return(domain.File{ID: "mine", Owner: "alice", Key: "files/mine"}, nil)
	idx.EXPECT().Get(gomock.Any(), "theirs").Return(domain.File{ID: "theirs", Owner: "bob", Key: "files/theirs"}, nil)

	var shortlist []domain.File
	retriever.EXPECT().Match(gomock.Any(), "invoice", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, files []domain.File) (retrieve.Answer, error) {
			shortlist = files
			return retrieve.Answer{IDs: []string{"mine"}}, nil
		})
	blobs.EXPECT().PresignGet(gomock.Any(), "files/mine", gomock.Any()).Return("https://dl", nil)

	req := withOwner(httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"invoice"}`)), "alice")
	rec := httptest.NewRecorder()

	// Act
	c.Ask(rec, req)

	// Assert: bob's file never reached the model, and the result set is alice's only.
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, shortlist, 1)
	assert.Equal(t, "mine", shortlist[0].ID)
	assert.NotContains(t, rec.Body.String(), "theirs")
}
