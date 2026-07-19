package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/api"
	agentmock "github.com/kazemisoroush/vault/backend/internal/agent/mock"
	"github.com/kazemisoroush/vault/backend/internal/config"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
	"github.com/kazemisoroush/vault/backend/internal/telemetry"
)

func TestNewFailsClosedWhenAuthNotConfigured(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	answerer := agentmock.NewMockAnswerer(ctrl)
	callLister := mocks.NewMockCallLister(ctrl)
	checkStore := mocks.NewMockCheckStore(ctrl)
	enqueuer := mocks.NewMockEnqueuer(ctrl)

	// Act
	_, err := api.NewHandler(context.Background(), config.Config{}, idx, blobs, answerer, checkStore, enqueuer, callLister, telemetry.NoopEmitter{})

	// Assert
	assert.Error(t, err)
}

func TestNewAuthDisabledServesDataRoute(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	idx := mocks.NewMockIndex(ctrl)
	blobs := mocks.NewMockStore(ctrl)
	answerer := agentmock.NewMockAnswerer(ctrl)
	callLister := mocks.NewMockCallLister(ctrl)
	idx.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, "", nil)

	// Act
	handler, err := api.NewHandler(context.Background(), config.Config{AuthDisabled: true}, idx, blobs, answerer, mocks.NewMockCheckStore(ctrl), mocks.NewMockEnqueuer(ctrl), callLister, telemetry.NoopEmitter{})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files", nil))

	// Assert
	assert.Equal(t, http.StatusOK, rec.Code)
}
