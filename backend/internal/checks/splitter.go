package checks

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// splitInstruction asks the model to break the text into atomic claims, kept in its own file so
// it can be read, edited, and evaluated on its own. Each claim must be a verbatim substring of
// the input, so the backend can locate its offsets deterministically instead of trusting
// model-produced numbers.
//
//go:embed prompts/split.prompt
var splitInstruction string

// splitMaxTokens must fit the claims of a maximum-size check text. The claims are substrings of
// the input plus JSON overhead, so the ceiling tracks maxCheckChars (20000 chars is roughly
// 5000 tokens) with headroom for quoting and punctuation.
const splitMaxTokens = 8192

// split asks the model for the text's atomic claims and locates each one's offsets in the text.
// A claim the model did not copy verbatim cannot be located and is dropped with a loud log: a
// claim without true offsets could never be highlighted or verified honestly.
func split(ctx context.Context, model Converser, text string) ([]domain.Claim, error) {
	prompt := fmt.Sprintf("%s\n\n---\n%s", splitInstruction, text)
	reply, err := model.Converse(ctx, llm.Conversation{
		Prompt:    prompt,
		MaxTokens: splitMaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("split claims: %w", err)
	}

	parts, err := parseClaimTexts(reply)
	if err != nil {
		return nil, fmt.Errorf("split claims: %w", err)
	}

	claims := make([]domain.Claim, 0, len(parts))
	cursor := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		at := strings.Index(text[cursor:], part)
		if at < 0 {
			// Out-of-order occurrences are still located from the top before giving up.
			at = strings.Index(text, part)
			if at < 0 {
				logDroppedClaim(part)
				continue
			}
		} else {
			at += cursor
		}
		claims = append(claims, domain.Claim{Text: part, Start: at, End: at + len(part)})
		cursor = at + len(part)
	}
	return claims, nil
}

// logDroppedClaim reports a non-verbatim claim without writing the user's text to the logs.
func logDroppedClaim(part string) {
	log.Printf("split: claim is not a verbatim substring, dropped (%d chars)", len(part))
}

// parseClaimTexts reads the model's JSON array of claim strings.
func parseClaimTexts(reply string) ([]string, error) {
	start := strings.Index(reply, "[")
	end := strings.LastIndex(reply, "]")
	if start < 0 || end < 0 || end < start {
		return nil, fmt.Errorf("no JSON array in model reply")
	}

	var parts []string
	if err := json.Unmarshal([]byte(reply[start:end+1]), &parts); err != nil {
		return nil, fmt.Errorf("decode claim list: %w", err)
	}
	return parts, nil
}
