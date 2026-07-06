package telemetry

// NoopEmitter discards metrics, for local runs and tests that do not assert on telemetry.
type NoopEmitter struct{}

// Emit does nothing.
func (NoopEmitter) Emit(_ string, _ map[string]string, _ ...Metric) {}
