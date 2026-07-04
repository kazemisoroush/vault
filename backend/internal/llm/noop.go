package llm

import "context"

// NoopRecorder discards calls; used where telemetry is not wired, such as the local server.
type NoopRecorder struct{}

// Record does nothing.
func (NoopRecorder) Record(context.Context, Call) {}
