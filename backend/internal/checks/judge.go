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

// judgeInstruction asks the model whether one claim is supported by the candidate documents,
// kept in its own file so it can be read, edited, and evaluated on its own. It tells the model
// the documents are untrusted data; the deterministic gate and the verbatim claim-span match in
// the runner are what actually make an injected instruction harmless to a green verdict.
//
//go:embed prompts/judge.prompt
var judgeInstruction string

// judgeMaxTokens caps the judge reply: one span and a tier.
const judgeMaxTokens = 1024

// maxDocChars bounds each candidate document's text in the judge prompt.
const maxDocChars = 30000

// candidate is one document offered to the judge.
type candidate struct {
	FileID   string
	FileName string
	Text     string
}

// judgement is the model's parsed answer for one claim.
type judgement struct {
	FileID string `json:"fileId"`
	Span   string `json:"span"`
	Tier   string `json:"tier"`
}

// judge asks the model for the best supporting span among the candidates. The reply is a
// proposal: the runner's gate decides what it is worth.
func judge(ctx context.Context, model Converser, claim string, candidates []candidate) (judgement, error) {
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
		return judgement{}, fmt.Errorf("judge claim: %w", err)
	}

	return parseJudgement(reply)
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

// parseJudgement reads the judge's JSON object, treating a malformed reply as tier none so a
// confused model can only ever produce an unsupported verdict, never a green.
func parseJudgement(reply string) (judgement, error) {
	start := strings.Index(reply, "{")
	end := strings.LastIndex(reply, "}")
	if start < 0 || end < 0 || end < start {
		return judgement{Tier: "none"}, nil
	}

	var j judgement
	if err := json.Unmarshal([]byte(reply[start:end+1]), &j); err != nil {
		return judgement{Tier: "none"}, nil
	}
	if j.Tier == "" {
		j.Tier = "none"
	}
	return j, nil
}
