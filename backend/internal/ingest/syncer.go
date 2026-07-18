package ingest

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/kb"
)

// syncBatch bounds how many landed files one sync advances. A run advances the files it snapshotted
// before starting the job; any landed files past the batch are drained by later runs.
const syncBatch = 100

// fileStatusIndex lists files by lifecycle status and writes them back, so the syncer can advance
// landed files to ingested. *index.DynamoIndex satisfies it; the interface keeps the syncer testable.
type fileStatusIndex interface {
	ListByStatus(ctx context.Context, status string, limit int32) ([]domain.File, error)
	Put(ctx context.Context, file domain.File) error
}

// Syncer advances landed files to ingested. Driven by a schedule, it snapshots the landed files,
// runs one Knowledge Base ingestion job over the data source, and on completion marks that snapshot
// ingested, so a file is searchable a short while after it lands.
type Syncer struct {
	indexer kb.Indexer
	index   fileStatusIndex
	now     func() time.Time
}

// NewSyncer builds a Syncer over the Knowledge Base indexer and the file index.
func NewSyncer(indexer kb.Indexer, index fileStatusIndex) *Syncer {
	return &Syncer{indexer: indexer, index: index, now: time.Now}
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
		file.Status = domain.StatusIngested
		file.UpdatedAt = s.now().UTC()
		if err := s.index.Put(ctx, file); err != nil {
			// One record failing to advance is not fatal: it stays landed and the next run retries.
			log.Printf("mark file %s ingested: %v", file.ID, err)
		}
	}
	return nil
}
