package telemetry

import (
	"encoding/json"
	"io"
	"log"
	"sort"
	"time"
)

// EMFEmitter writes metrics as CloudWatch Embedded Metric Format JSON lines to a writer.
type EMFEmitter struct {
	out io.Writer
	now func() time.Time
}

// NewEMFEmitter builds an EMFEmitter over a writer, typically os.Stdout under Lambda.
func NewEMFEmitter(out io.Writer) *EMFEmitter {
	return &EMFEmitter{out: out, now: time.Now}
}

// Emit writes one EMF record so CloudWatch extracts each metric under the namespace and dimensions.
func (e *EMFEmitter) Emit(namespace string, dimensions map[string]string, metrics ...Metric) {
	if len(metrics) == 0 {
		return
	}

	dimensionKeys := make([]string, 0, len(dimensions))
	for key := range dimensions {
		dimensionKeys = append(dimensionKeys, key)
	}
	sort.Strings(dimensionKeys)

	definitions := make([]map[string]string, 0, len(metrics))
	for _, metric := range metrics {
		definitions = append(definitions, map[string]string{"Name": metric.Name, "Unit": metric.Unit})
	}

	record := map[string]any{
		"_aws": map[string]any{
			"Timestamp": e.now().UnixMilli(),
			"CloudWatchMetrics": []map[string]any{{
				"Namespace":  namespace,
				"Dimensions": [][]string{dimensionKeys},
				"Metrics":    definitions,
			}},
		},
	}
	for key, value := range dimensions {
		record[key] = value
	}
	for _, metric := range metrics {
		record[metric.Name] = metric.Value
	}

	line, err := json.Marshal(record)
	if err != nil {
		log.Printf("marshal EMF metric: %v", err)
		return
	}
	if _, err := e.out.Write(append(line, '\n')); err != nil {
		log.Printf("write EMF metric: %v", err)
	}
}
