package kb

import "context"

//go:generate go tool mockgen -source=indexer.go -destination=../mocks/indexer_mock.go -package=mocks

// Indexer drives the managed data source's ingestion so landed files become searchable. Sync starts
// one ingestion job and waits for it, returning true when a job it started completed. *BedrockIndexer
// satisfies it; the interface lets the syncer be tested without the Bedrock client.
type Indexer interface {
	Sync(ctx context.Context) (bool, error)
}
