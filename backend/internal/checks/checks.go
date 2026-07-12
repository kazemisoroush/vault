// Package checks verifies pasted text against the owner's stored documents. A model proposes
// claims and supporting spans; pure code confirms every span against the stored text before a
// claim may be called verified. The north star is zero false greens: no verdict path can mark a
// claim verified without the gate's character-for-character match.
package checks

import (
	"context"
	"errors"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/llm"
)

//go:generate go tool mockgen -source=checks.go -destination=../mocks/checks_mock.go -package=mocks -mock_names=Store=MockCheckStore,Converser=MockConverser,Enqueuer=MockEnqueuer

// ErrNotFound is returned when a check does not exist.
var ErrNotFound = errors.New("check not found")

// ModelOp is the operation label the check pipeline's model calls carry on the trace.
const ModelOp = "check"

// Store persists checks.
type Store interface {
	Put(ctx context.Context, check domain.Check) error
	// Get returns a check by id without an ownership check, so a caller serving a user must verify it.
	Get(ctx context.Context, id string) (domain.Check, error)
}

// Converser is the one slice of the model the pipeline uses: a prompt in, text out.
type Converser interface {
	Converse(ctx context.Context, conv llm.Conversation) (string, error)
}

// Enqueuer hands a created check to the worker that runs its pipeline asynchronously.
type Enqueuer interface {
	Enqueue(ctx context.Context, checkID string, ownerID string) error
}
