package kb

import "context"

//go:generate go tool mockgen -source=searcher.go -destination=../mocks/searcher_mock.go -package=mocks

// Searcher finds the passages most relevant to a query in the Knowledge Base by hybrid search.
// *BedrockSearcher satisfies it; the interface lets a consumer (the question answerer, the check
// verifier) depend on the search capability alone and be tested with a fake, without importing the
// Bedrock client.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]Passage, error)
}
