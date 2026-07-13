package checks

import "strings"

// The gate is the product's reason to exist, and it is deliberately dumb: no model, no
// normalization, no fuzz. A span either occurs in the stored text exactly, character for
// character, or it does not. Anything cleverer would open the door to a false green, so
// near-misses (different unicode spaces, curly quotes, OCR drift) fail closed: the claim
// stays unsupported rather than gaining a verified reference.

// Locate returns the byte offsets of the first exact occurrence of span in text. It reports
// ok=false when the span is empty or does not occur, so a fabricated span can never gain offsets.
func Locate(text string, span string) (start int, end int, ok bool) {
	if span == "" {
		return 0, 0, false
	}
	at := strings.Index(text, span)
	if at < 0 {
		return 0, 0, false
	}
	return at, at + len(span), true
}

// Verify reports whether text[start:end] is exactly span. It is the final check before a claim
// may be called verified: offsets out of range or a single differing byte fail it.
func Verify(text string, span string, start int, end int) bool {
	if span == "" || start < 0 || end < start || end > len(text) {
		return false
	}
	return text[start:end] == span
}
