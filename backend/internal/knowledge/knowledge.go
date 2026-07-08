// Package knowledge is Vault's knowledge base: one owner-scoped view over the file records
// that answers three ways. Search finds files by meaning, Query finds them by their normalised
// attributes, and Fetch returns one file with the text known about it. An agent uses these as
// tools to answer a question without the caller touching the vector store or the index directly.
package knowledge

import (
	"context"
	"time"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

// The Base mock lives in its own package, not the shared mocks package. knowledge imports the
// low-level embed, vectors, and index packages, whose own tests use the shared mocks, so a Base
// mock in that shared package would form an import cycle. A dedicated package avoids it.
//go:generate go tool mockgen -source=knowledge.go -destination=mock/base_mock.go -package=knowledgemock

// Base is the knowledge base. Every call is scoped to one owner and never returns another
// owner's files.
type Base interface {
	// Search returns the owner's files nearest to the query text by meaning, closest first,
	// then keeps only those that also pass the filter. topK bounds the shortlist it starts from.
	Search(ctx context.Context, ownerID, query string, filter Filter, topK int) ([]domain.File, error)
	// Query returns the owner's files that match the filter, without semantic search.
	Query(ctx context.Context, ownerID string, filter Filter) ([]domain.File, error)
	// Fetch returns one file the owner owns, with the text the knowledge base holds about it.
	Fetch(ctx context.Context, ownerID, id string) (Document, error)
}

// Filter narrows files by their normalised attributes and their creation time. A zero field is
// not applied, so the zero Filter matches everything. Text fields match without case sensitivity
// and as a substring, so "sara" matches the person "Sara Jabbari".
type Filter struct {
	Person  string
	DocType string
	Vendor  string
	// Since and Until bound CreatedAt. A zero time means no bound on that side.
	Since time.Time
	Until time.Time
}

// Document is one file and the text the knowledge base holds about it, which is its name and
// its extracted metadata joined together.
type Document struct {
	File domain.File
	Text string
}
