// Package chunk splits a file into the short texts that get embedded for search, one vector each.
package chunk

import (
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	// maxPassageRunes bounds a body passage before a new chunk starts, so a single fact stays on
	// its own vector instead of being averaged into a whole-document summary.
	maxPassageRunes = 512
	// maxChunks caps the chunks per file, so a very large document cannot explode the vector index.
	maxChunks = 64
)

// Chunks builds the texts embedded for a file, each becoming its own vector: the file name, every
// metadata field on its own, and the body split into passages. Splitting keeps a single fact (a
// passport number in its field, a line in the body) on its own point in space, so a query matches
// the closest chunk instead of one blurred whole-file vector. The result is deduplicated and
// capped, and is stable for the same input because metadata keys are visited in sorted order.
func Chunks(name string, meta map[string]string, text string) []string {
	chunks := make([]string, 0, len(meta)+8)
	seen := make(map[string]struct{}, len(meta)+8)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		chunks = append(chunks, s)
	}

	add(name)

	keys := make([]string, 0, len(meta))
	for key := range meta {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		add(key + ": " + meta[key])
	}

	for _, passage := range passages(text) {
		add(passage)
	}

	if len(chunks) > maxChunks {
		chunks = chunks[:maxChunks]
	}
	return chunks
}

// passages splits body text into runs of at most maxPassageRunes, breaking first on blank lines
// and then on line boundaries, so a chunk stays a coherent run of text rather than a fixed byte
// window. A single line longer than the budget is hard-split by runes so no chunk exceeds the cap.
func passages(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var out []string
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			out = append(out, strings.TrimSpace(buf.String()))
			buf.Reset()
		}
	}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if buf.Len() > 0 && utf8.RuneCountInString(buf.String())+1+utf8.RuneCountInString(line) > maxPassageRunes {
			flush()
		}
		for utf8.RuneCountInString(line) > maxPassageRunes {
			runes := []rune(line)
			out = append(out, string(runes[:maxPassageRunes]))
			line = string(runes[maxPassageRunes:])
		}
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(line)
	}
	flush()
	return out
}
