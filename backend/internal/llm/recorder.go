package llm

import "context"

//go:generate go tool mockgen -source=recorder.go -destination=../mocks/recorder_mock.go -package=mocks

// Recorder stores a completed LLM call. Recording must never fail the operation, so it
// returns nothing and the implementation handles its own errors.
type Recorder interface {
	Record(ctx context.Context, call Call)
}
