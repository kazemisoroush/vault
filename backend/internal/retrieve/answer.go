package retrieve

// Answer is the model's response to an ask: a human-readable answer and the matching file ids.
type Answer struct {
	// Text is a short answer drawn from the matched files' metadata, empty when the query is a
	// plain find or the metadata does not answer it.
	Text string
	// IDs are the matching file ids, most relevant first.
	IDs []string
}
