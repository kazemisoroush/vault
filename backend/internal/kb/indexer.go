package kb

import "context"

//go:generate go tool mockgen -source=indexer.go -destination=../mocks/indexer_mock.go -package=mocks

// SyncResult is the outcome of one ingestion sync: whether a job it started completed, and the ids
// of the files that job could not index, so the caller can mark those failed rather than searchable.
type SyncResult struct {
	Completed     bool
	FailedFileIDs []string
}

// Indexer drives the managed data source's ingestion so landed files become searchable. Sync starts
// one ingestion job and waits for it. *BedrockIndexer satisfies it; the interface lets the syncer be
// tested without the Bedrock client.
type Indexer interface {
	Sync(ctx context.Context) (SyncResult, error)
}
