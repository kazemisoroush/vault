package checks

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

// The splitter is pure code, no model. Sentences are the unit of checking: every character of
// the input belongs to exactly one segment, so full coverage holds by construction and nothing
// can be silently skipped. An imperfect boundary only makes an awkward claim, never a false
// green, so the heuristics stay small and legible.

// abbreviations are dot-ended tokens that do not close a sentence. The list leans legal
// (v. between party names, cl. and s. for clauses and sections) plus everyday honorifics.
var abbreviations = map[string]bool{
	"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true,
	"v": true, "vs": true, "no": true, "cl": true, "s": true, "ss": true,
	"art": true, "para": true, "p": true, "pp": true, "approx": true,
	"e.g": true, "i.e": true, "etc": true, "cf": true,
}

// split breaks text into claims at sentence boundaries: runs of . ! ? and every newline (a
// heading or list item rarely ends with punctuation but still deserves a verdict). A dot does
// not close a sentence after a known abbreviation or a single-letter initial, inside a number,
// or when the text continues in lowercase. Segments with no letter or digit (stray punctuation,
// rules) are dropped; they assert nothing checkable.
func split(text string) []domain.Claim {
	var claims []domain.Claim
	segStart := 0

	flush := func(end int) {
		if claim, ok := claimFromSegment(text, segStart, end); ok {
			claims = append(claims, claim)
		}
		segStart = end
	}

	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '.', '!', '?':
			end := i
			for end < len(text) && isSentenceEnder(text[end]) {
				end++
			}
			end = absorbClosingQuotes(text, end)
			if text[i] == '.' && !dotClosesSentence(text[:i], text[end:]) {
				i = end - 1
				continue
			}
			flush(end)
			i = end - 1
		case '\n':
			// A newline closes whatever came before it: headings and list items rarely end
			// with punctuation but still deserve a verdict.
			flush(i)
		}
	}
	flush(len(text))
	return claims
}

// isSentenceEnder reports whether the byte extends a sentence-ending run, so "?!" and "..."
// close one sentence, not several, and a closing quote or bracket stays attached.
func isSentenceEnder(b byte) bool {
	switch b {
	case '.', '!', '?', '"', '\'', ')', ']':
		return true
	default:
		return false
	}
}

// dotClosesSentence decides whether a dot is a real sentence boundary given what precedes and
// follows it. Decimal points, clause numbers, abbreviations, single-letter initials, and dots
// followed by a lowercase continuation do not close sentences.
func dotClosesSentence(before string, after string) bool {
	// A dot with digits immediately on both sides is a decimal or clause number, never a
	// boundary. Immediately: "$40,000. 14 days later" is two sentences.
	if len(before) > 0 && isDigit(before[len(before)-1]) && after != "" && isDigit(after[0]) {
		return false
	}

	word := lastWord(before)
	if abbreviations[strings.ToLower(word)] {
		return false
	}
	// A single letter is an initial ("J. Rossi") or the first half of a dotted abbreviation
	// ("e.g." seen at its first dot), either way not a boundary.
	if len(word) == 1 && unicode.IsLetter(rune(word[0])) {
		return false
	}
	// Sentences do not start lowercase: "agreed, e.g. in cl. 3" continues after the dot.
	if r, ok := nextLetterOrDigit(after); ok && unicode.IsLower(r) {
		return false
	}
	return true
}

// nextLetterOrDigit returns the first letter or digit after any spaces or tabs, so the
// lowercase-continuation guard looks at the actual next word.
func nextLetterOrDigit(after string) (rune, bool) {
	for _, r := range after {
		switch {
		case r == ' ' || r == '\t':
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			return r, true
		default:
			return 0, false
		}
	}
	return 0, false
}

// absorbClosingQuotes extends a sentence-ender run over typographic closers, which are
// multi-byte and invisible to the byte-level scan, so a curly quote stays with its sentence.
func absorbClosingQuotes(text string, end int) int {
	for end < len(text) {
		r, size := utf8.DecodeRuneInString(text[end:])
		switch r {
		case '\u201d', '\u2019', '\u00bb', '\u203a':
			end += size
		default:
			return end
		}
	}
	return end
}

// lastWord returns the token immediately before the dot, inner dots kept so "e.g" matches.
func lastWord(text string) string {
	end := len(text)
	start := end
	for start > 0 {
		c := text[start-1]
		if c == ' ' || c == '\t' || c == '\n' || c == '(' || c == '"' {
			break
		}
		start--
	}
	return text[start:end]
}

// isDigit reports whether the byte is an ASCII digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// claimFromSegment trims a raw segment to its checkable core, keeping offsets exact. A segment
// with no letter or digit asserts nothing and is skipped.
func claimFromSegment(text string, start int, end int) (domain.Claim, bool) {
	for start < end && isTrimmable(text[start]) {
		start++
	}
	for end > start && isTrimmable(text[end-1]) {
		end--
	}
	segment := text[start:end]
	if !strings.ContainsFunc(segment, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return domain.Claim{}, false
	}
	return domain.Claim{Text: segment, Start: start, End: end}, true
}

// isTrimmable reports whether a byte is segment-edge whitespace.
func isTrimmable(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
