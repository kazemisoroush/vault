package retrieve

// Answer is the model's response to an ask: a human-readable answer and the matching file ids.
type Answer struct {
	// Text is a short answer drawn from the matched files' metadata, empty when there is none.
	Text string
	// IDs are the matching file ids, most relevant first; when Text is set, IDs[0] is its source.
	IDs []string
}
