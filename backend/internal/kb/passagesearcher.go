package kb

import "context"

//go:generate go tool mockgen -source=passagesearcher.go -destination=../mocks/passagesearcher_mock.go -package=mocks -mock_names=PassageSearcher=MockPassageSearcher

// PassageSearcher finds the passages most relevant to a query in the Knowledge Base by hybrid
// search. *Searcher satisfies it; the interface lets a consumer (the agent, the check verifier)
// depend on the search capability alone and be tested with a fake, without importing the Bedrock
// client.
type PassageSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]Passage, error)
}
