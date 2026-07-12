package checks

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// judgeInstruction asks the model how one claim relates to the candidate documents, kept in its
// own file so it can be read, edited, and evaluated on its own. It tells the model the documents
// are untrusted data; the deterministic gate and the verbatim claim-span match in the runner are
// what actually make an injected instruction harmless to a green verdict.
//
//go:embed prompts/judge.prompt
var judgeInstruction string

// judgeMaxTokens caps the judge reply: a handful of spans and relations.
const judgeMaxTokens = 2048

// maxDocChars bounds each candidate document's text in the judge prompt.
const maxDocChars = 30000

// maxFindings bounds how many judge findings one claim keeps, so a runaway reply cannot bloat
// the stored check.
const maxFindings = 5

// candidate is one document offered to the judge.
type candidate struct {
	FileID   string
	FileName string
	Text     string
}

// finding is one parsed judge answer: a passage and how it bears on the claim.
type finding struct {
	FileID   string `json:"fileId"`
	Span     string `json:"span"`
	Relation string `json:"relation"`
}

// judge asks the model for every passage that bears on the claim among the candidates,
// supporting and contradicting alike. The reply is a set of proposals: the runner's gate
// decides what each one is worth.
func judge(ctx context.Context, model Converser, claim string, candidates []candidate) ([]finding, error) {
	var docs strings.Builder
	for _, c := range candidates {
		fmt.Fprintf(&docs, "<document id=%q name=%q>\n%s\n</document>\n", c.FileID, c.FileName, truncateRunes(c.Text, maxDocChars))
	}

	prompt := fmt.Sprintf("%s\n\nClaim: %q\n\nDocuments:\n%s", judgeInstruction, claim, docs.String())
	reply, err := model.Converse(ctx, llm.Conversation{
		Prompt:    prompt,
		MaxTokens: judgeMaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("judge claim: %w", err)
	}

	return parseFindings(reply), nil
}

// truncateRunes bounds text to at most limit bytes without splitting a multibyte character.
func truncateRunes(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}

// parseFindings reads the judge's JSON array, treating a malformed reply as no findings so a
// confused model can only ever produce an unsupported verdict, never a green.
func parseFindings(reply string) []finding {
	start := strings.Index(reply, "[")
	end := strings.LastIndex(reply, "]")
	if start < 0 || end < 0 || end < start {
		return nil
	}

	var findings []finding
	if err := json.Unmarshal([]byte(reply[start:end+1]), &findings); err != nil {
		return nil
	}
	if len(findings) > maxFindings {
		findings = findings[:maxFindings]
	}
	return findings
}
