package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// judgeInstruction asks the model whether one claim is supported by the candidate documents. The
// span must be copied exactly: the gate re-reads the document at the span's location, so a
// paraphrased "quote" is caught and discarded, never softened.
const judgeInstruction = `You are checking whether a claim is supported by the documents below.
Return ONLY a JSON object: {"fileId": "...", "span": "...", "tier": "..."}.
"span" is the single passage that best supports the claim, COPIED EXACTLY, character for
character, from one document. "fileId" is that document's id.
"tier" is one of:
  "verbatim":   the span states the claim in the document's own words.
  "paraphrase": the span supports the claim but in different words.
  "none":       no document supports the claim; then omit fileId and span.
Never invent or adjust a span. If unsure, choose "none".`

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
		text := c.Text
		if len(text) > maxDocChars {
			text = text[:maxDocChars]
		}
		fmt.Fprintf(&docs, "<document id=%q name=%q>\n%s\n</document>\n", c.FileID, c.FileName, text)
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
