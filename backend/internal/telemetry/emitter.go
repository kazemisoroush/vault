// Package telemetry emits application metrics as CloudWatch EMF log lines, which CloudWatch
// extracts into metrics with no agent or extra infrastructure.
package telemetry

//go:generate go tool mockgen -source=emitter.go -destination=../mocks/emitter_mock.go -package=mocks

// Metric is one named measurement and its unit.
type Metric struct {
	Name  string
	Value float64
	Unit  string
}

// Emitter records metrics that share a namespace and dimensions as one measurement.
type Emitter interface {
	Emit(namespace string, dimensions map[string]string, metrics ...Metric)
}
