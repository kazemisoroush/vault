package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// splitInstruction asks the model to break the text into atomic claims. Each claim must be a
// verbatim substring of the input, so the backend can locate its offsets deterministically
// instead of trusting model-produced numbers.
const splitInstruction = `Split the text below into atomic factual claims, one checkable assertion each.
A sentence carrying two facts becomes two claims. COPY each claim exactly, character for
character, as it appears in the text; never rephrase, merge, or correct. Skip headings,
greetings, and sentences that assert nothing. Return ONLY a JSON array of strings, in order
of appearance. No commentary.`

// splitMaxTokens leaves room for the claims of a full pasted page.
const splitMaxTokens = 4096

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
				log.Printf("split: claim is not a verbatim substring, dropped: %q", part)
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
