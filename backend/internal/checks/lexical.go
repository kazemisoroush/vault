package checks

import (
	"context"
	"log"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
)

const (
	// minTokenRunes is the shortest claim token treated as a distinctive exact value. It skips short
	// numbers and years and keeps identifiers like a passport or invoice number, which a meaning
	// search cannot rank but a literal match nails.
	minTokenRunes = 5
	// maxLexical bounds the literal-match files fed to the judge for one claim.
	maxLexical = 8
	// lexPageSize is the page size for listing an owner's files during a literal scan.
	lexPageSize = int32(100)
)

// lexicalCandidates finds the owner's files whose name or metadata literally contains a distinctive
// token from the claim, such as an identifier a meaning search cannot rank. It complements the
// vector candidates: retrieval by meaning misses exact values, and the gate still verifies every
// span, so a literal match can only add real support, never a false green. It returns nil when the
// claim carries no such token, so an ordinary sentence pays no extra cost.
func (r *Runner) lexicalCandidates(ctx context.Context, ownerID string, claim string) []candidate {
	tokens := distinctiveTokens(claim)
	if len(tokens) == 0 {
		return nil
	}

	out := make([]candidate, 0, maxLexical)
	cursor := ""
	for {
		page, next, err := r.index.List(ctx, ownerID, lexPageSize, cursor)
		if err != nil {
			log.Printf("lexical list for check: %v", err)
			return out
		}
		for _, file := range page {
			if !recordContainsToken(file, tokens) {
				continue
			}
			text, _, err := r.blobs.Get(ctx, blob.TextKey(file.ID))
			if err != nil || len(text) == 0 {
				continue
			}
			out = append(out, candidate{FileID: file.ID, FileName: file.Name, Text: string(text)})
			if len(out) >= maxLexical {
				return out
			}
		}
		if next == "" {
			return out
		}
		cursor = next
	}
}

// distinctiveTokens pulls the claim's distinctive exact values: alphanumeric tokens of at least
// minTokenRunes that contain a digit, lowercased and deduplicated. These are the identifiers a
// meaning-based search cannot rank but a literal match can.
func distinctiveTokens(claim string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, field := range strings.FieldsFunc(claim, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if utf8.RuneCountInString(field) < minTokenRunes || !hasDigit(field) {
			continue
		}
		token := strings.ToLower(field)
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

// recordContainsToken reports whether the file's name or any metadata value contains a token. Tokens
// are already lowercased, so the haystack is lowered once for a case-insensitive literal match.
func recordContainsToken(file domain.File, tokens []string) bool {
	var hay strings.Builder
	hay.WriteString(strings.ToLower(file.Name))
	for _, value := range file.Meta {
		hay.WriteByte('\n')
		hay.WriteString(strings.ToLower(value))
	}
	haystack := hay.String()
	for _, token := range tokens {
		if strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}

// hasDigit reports whether s contains a digit, which marks a token as an identifier, not a word.
func hasDigit(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
