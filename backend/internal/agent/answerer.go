package agent

import (
	"context"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

//go:generate go tool mockgen -source=answerer.go -destination=mock/agent_mock.go -package=agentmock

// Answerer answers a query for one owner. The controller depends on this, not the concrete agent.
type Answerer interface {
	Answer(ctx context.Context, ownerID, query string) (Result, error)
}

// Converser runs one tool-using exchange with the model. The concrete *llm.Model satisfies it.
type Converser interface {
	Converse(ctx context.Context, c llm.Conversation) (string, error)
}
