package ingest

import (
	"context"
	"fmt"
	"log"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/kb"
)

// syncBatch bounds how many landed files one sync advances. A run advances the files it snapshotted
// before starting the job; any landed files past the batch are drained by later runs.
const syncBatch = 100

// fileStatusIndex lists files by lifecycle status and advances a file's status, so the syncer can
// move landed files to ingested. *index.DynamoIndex satisfies it; the interface keeps the syncer
// testable.
type fileStatusIndex interface {
	ListByStatus(ctx context.Context, status string, limit int32) ([]domain.File, error)
	AdvanceStatus(ctx context.Context, id string, from string, to string) error
}

// Syncer advances landed files to ingested. Driven by a schedule, it snapshots the landed files,
// runs one Knowledge Base ingestion job over the data source, and on completion advances that
// snapshot to ingested, so a file is searchable a short while after it lands.
type Syncer struct {
	indexer kb.Indexer
	index   fileStatusIndex
}

// NewSyncer builds a Syncer over the Knowledge Base indexer and the file index.
func NewSyncer(indexer kb.Indexer, index fileStatusIndex) *Syncer {
	return &Syncer{indexer: indexer, index: index}
}

// Sync advances a batch of landed files to ingested once the Knowledge Base has indexed them. The
// landed files are snapshotted before the job starts, so a file that lands mid-job is left for the
// next run rather than marked ingested before it is actually in the index.
func (s *Syncer) Sync(ctx context.Context) error {
	landed, err := s.index.ListByStatus(ctx, domain.StatusLanded, syncBatch)
	if err != nil {
		return fmt.Errorf("list landed files: %w", err)
	}
	if len(landed) == 0 {
		return nil // nothing new to index
	}

	completed, err := s.indexer.Sync(ctx)
	if err != nil {
		return fmt.Errorf("run ingestion sync: %w", err)
	}
	if !completed {
		return nil // a job was already running; a later run advances these files
	}

	for _, file := range landed {
		// AdvanceStatus is conditional, so a file deleted or changed since the snapshot is skipped
		// rather than resurrected. One record failing is not fatal: it stays landed and the next
		// run retries.
		if err := s.index.AdvanceStatus(ctx, file.ID, domain.StatusLanded, domain.StatusIngested); err != nil {
			log.Printf("advance file %s to ingested: %v", file.ID, err)
		}
	}
	return nil
}
