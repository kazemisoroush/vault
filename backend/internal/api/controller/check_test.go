package controller_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/api/controller"
	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

// asOwner stamps the request context with an authenticated owner, as the auth middleware would.
func asOwner(r *http.Request, ownerID string) *http.Request {
	return r.WithContext(auth.WithOwnerID(r.Context(), ownerID))
}

func TestCreateCheckPersistsPendingAndEnqueues(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	store := mocks.NewMockCheckStore(ctrl)
	enqueuer := mocks.NewMockEnqueuer(ctrl)

	var saved domain.Check
	store.EXPECT().Put(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, c domain.Check) error {
		saved = c
		return nil
	})
	enqueuer.EXPECT().Enqueue(gomock.Any(), gomock.Any(), "alice").Return(nil)

	c := controller.NewCheckController(store, enqueuer)
	rec := httptest.NewRecorder()
	req := asOwner(httptest.NewRequest(http.MethodPost, "/checks", strings.NewReader(`{"text":"The deposit was paid."}`)), "alice")

	// Act
	c.Create(rec, req)

	// Assert
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, domain.CheckPending, saved.Status)
	assert.Equal(t, "alice", saved.OwnerID)
	assert.Equal(t, "The deposit was paid.", saved.Text)
}

func TestCreateCheckRejectsEmptyText(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	c := controller.NewCheckController(mocks.NewMockCheckStore(ctrl), mocks.NewMockEnqueuer(ctrl))
	rec := httptest.NewRecorder()

	// Act
	c.Create(rec, asOwner(httptest.NewRequest(http.MethodPost, "/checks", strings.NewReader(`{"text":"  "}`)), "alice"))

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateCheckRejectsOversizedText(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	c := controller.NewCheckController(mocks.NewMockCheckStore(ctrl), mocks.NewMockEnqueuer(ctrl))
	rec := httptest.NewRecorder()
	huge := strings.Repeat("a", 20001)

	// Act
	c.Create(rec, asOwner(httptest.NewRequest(http.MethodPost, "/checks", strings.NewReader(`{"text":"`+huge+`"}`)), "alice"))

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetCheckHidesForeignCheckAsNotFound(t *testing.T) {
	// Arrange: the check exists but belongs to someone else; the caller must see a plain 404.
	ctrl := gomock.NewController(t)
	store := mocks.NewMockCheckStore(ctrl)
	store.EXPECT().Get(gomock.Any(), "chk-1").Return(domain.Check{ID: "chk-1", OwnerID: "alice"}, nil)

	c := controller.NewCheckController(store, mocks.NewMockEnqueuer(ctrl))
	rec := httptest.NewRecorder()
	req := asOwner(httptest.NewRequest(http.MethodGet, "/checks/chk-1", nil), "mallory")
	req.SetPathValue("id", "chk-1")

	// Act
	c.Get(rec, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetCheckReturnsOwnersCheck(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	store := mocks.NewMockCheckStore(ctrl)
	store.EXPECT().Get(gomock.Any(), "chk-1").Return(domain.Check{ID: "chk-1", OwnerID: "alice", Status: domain.CheckDone}, nil)

	c := controller.NewCheckController(store, mocks.NewMockEnqueuer(ctrl))
	rec := httptest.NewRecorder()
	req := asOwner(httptest.NewRequest(http.MethodGet, "/checks/chk-1", nil), "alice")
	req.SetPathValue("id", "chk-1")

	// Act
	c.Get(rec, req)

	// Assert
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"done"`)
}
