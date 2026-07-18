package ingest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/ingest"
	"github.com/kazemisoroush/vault/backend/internal/mocks"
)

func TestSyncAdvancesLandedToIngestedWhenJobCompletes(t *testing.T) {
	// Arrange: two landed files and an ingestion job that completes.
	ctrl := gomock.NewController(t)
	indexer := mocks.NewMockIndexer(ctrl)
	idx := mocks.NewMockIndex(ctrl)

	landed := []domain.File{
		{ID: "a", Status: domain.StatusLanded},
		{ID: "b", Status: domain.StatusLanded},
	}
	idx.EXPECT().ListByStatus(gomock.Any(), domain.StatusLanded, gomock.Any()).Return(landed, nil)
	indexer.EXPECT().Sync(gomock.Any()).Return(true, nil)
	// Each snapshotted file is advanced landed -> ingested with a conditional write.
	idx.EXPECT().AdvanceStatus(gomock.Any(), "a", domain.StatusLanded, domain.StatusIngested).Return(nil)
	idx.EXPECT().AdvanceStatus(gomock.Any(), "b", domain.StatusLanded, domain.StatusIngested).Return(nil)

	s := ingest.NewSyncer(indexer, idx)

	// Act & Assert: both files advance; the strict mock fails on any unexpected call.
	require.NoError(t, s.Sync(context.Background()))
}

func TestSyncWithNoLandedFilesStartsNoJob(t *testing.T) {
	// Arrange: nothing landed, so no ingestion job runs.
	ctrl := gomock.NewController(t)
	indexer := mocks.NewMockIndexer(ctrl)
	idx := mocks.NewMockIndex(ctrl)
	idx.EXPECT().ListByStatus(gomock.Any(), domain.StatusLanded, gomock.Any()).Return(nil, nil)

	s := ingest.NewSyncer(indexer, idx)

	// Act & Assert: no indexer.Sync, no Put; the empty mocks would fail on any unexpected call.
	require.NoError(t, s.Sync(context.Background()))
}

func TestSyncLeavesLandedWhenAJobIsAlreadyRunning(t *testing.T) {
	// Arrange: a job is already running, so Sync reports it did not complete one.
	ctrl := gomock.NewController(t)
	indexer := mocks.NewMockIndexer(ctrl)
	idx := mocks.NewMockIndex(ctrl)

	idx.EXPECT().ListByStatus(gomock.Any(), domain.StatusLanded, gomock.Any()).
		Return([]domain.File{{ID: "a", Status: domain.StatusLanded}}, nil)
	indexer.EXPECT().Sync(gomock.Any()).Return(false, nil)

	s := ingest.NewSyncer(indexer, idx)

	// Act & Assert: the file stays landed (no Put), a later run advances it.
	require.NoError(t, s.Sync(context.Background()))
}

func TestSyncReturnsAnIngestionError(t *testing.T) {
	// Arrange: the ingestion job errors.
	ctrl := gomock.NewController(t)
	indexer := mocks.NewMockIndexer(ctrl)
	idx := mocks.NewMockIndex(ctrl)

	idx.EXPECT().ListByStatus(gomock.Any(), domain.StatusLanded, gomock.Any()).
		Return([]domain.File{{ID: "a", Status: domain.StatusLanded}}, nil)
	indexer.EXPECT().Sync(gomock.Any()).Return(false, errors.New("bedrock down"))

	s := ingest.NewSyncer(indexer, idx)

	// Act & Assert
	assert.Error(t, s.Sync(context.Background()))
}
