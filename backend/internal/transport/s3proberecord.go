package transport

// s3EventProbeRecord carries one record's event source.
type s3EventProbeRecord struct {
	EventSource string `json:"eventSource"`
}
