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

	"github.com/kazemisoroush/vault/backend/internal/agent"
	agentmock "github.com/kazemisoroush/vault/backend/internal/agent/mock"
	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// withOwner returns the request carrying an authenticated owner, as the auth middleware would.
func withOwner(r *http.Request, owner string) *http.Request {
	return r.WithContext(auth.WithOwnerID(r.Context(), owner))
}

func TestDropStampsTheOwner(t *testing.T) {
	// Arrange
	idx, blobs := mockDeps(t)
	c := NewFileController(idx, blobs)
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
	assert.Equal(t, "alice", saved.OwnerID)
}

func TestGetHidesAnotherOwnersFile(t *testing.T) {
	// Arrange: the record exists but belongs to bob; alice must see a 404, not a 403.
	idx, blobs := mockDeps(t)
	c := NewFileController(idx, blobs)
	idx.EXPECT().Get(gomock.Any(), "f1").Return(domain.File{ID: "f1", OwnerID: "bob", Key: "files/f1"}, nil)
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
	idx, blobs := mockDeps(t)
	c := NewFileController(idx, blobs)
	idx.EXPECT().List(gomock.Any(), "alice", gomock.Any(), gomock.Any()).Return(nil, "", nil)
	req := withOwner(httptest.NewRequest(http.MethodGet, "/files", nil), "alice")

	// Act
	c.List(httptest.NewRecorder(), req)

	// Assert: passing the owner to the index is the whole point; a nil return is fine.
}

func TestAskPassesTheCallersOwnerToTheAgent(t *testing.T) {
	// Arrange: the controller must hand the agent the authenticated caller, since every store
	// query the agent runs is scoped to that owner. The scoping itself is tested in the agent.
	ctrl := gomock.NewController(t)
	answerer := agentmock.NewMockAnswerer(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	c := NewAskController(answerer, blobs)

	var gotOwner string
	answerer.EXPECT().Answer(gomock.Any(), "alice", "invoice").DoAndReturn(
		func(_ context.Context, ownerID, _ string) (agent.Result, error) {
			gotOwner = ownerID
			return agent.Result{}, nil
		})
	req := withOwner(httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader(`{"query":"invoice"}`)), "alice")
	rec := httptest.NewRecorder()

	// Act
	c.Ask(rec, req)

	// Assert
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "alice", gotOwner)
}
