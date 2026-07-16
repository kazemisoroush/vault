package evals

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/llm"
)

//go:embed cases/*.json
var caseFS embed.FS

// Configuration for the Bedrock eval, read from the environment so it can run against a chosen
// region and model without code changes.
const (
	// evalModelOp tags the eval's model calls on the trace.
	evalModelOp = "eval"
	// envEvalBedrock gates the Bedrock eval; it runs only when this is set.
	envEvalBedrock = "VAULT_EVAL_BEDROCK"
	// envEvalRegion and envEvalModel override the Bedrock region and model.
	envEvalRegion = "VAULT_BEDROCK_REGION"
	envEvalModel  = "VAULT_EVAL_MODEL"
	// defaultEvalRegion and defaultEvalModel are used when the overrides are unset.
	defaultEvalRegion = "us-east-1"
	defaultEvalModel  = "us.anthropic.claude-haiku-4-5-20251001-v1:0"
)

// Case is one golden eval: a seeded vault, the question, a scripted tool run for the offline
// model, and the assertions the answer must meet. The real-model runner ignores Script.
type Case struct {
	Name   string     `json:"name"`
	Query  string     `json:"query"`
	Owner  string     `json:"owner"`
	Files  []CaseFile `json:"files"`
	Script CaseScript `json:"script"`
	Expect CaseExpect `json:"expect"`
}

// CaseFile is one file to seed into the vault before the case runs.
type CaseFile struct {
	ID   string            `json:"id"`
	Name string            `json:"name"`
	Meta map[string]string `json:"meta"`
}

// CaseScript is the tool run the offline model replays: each call, then the final reply.
type CaseScript struct {
	Calls []CaseCall `json:"calls"`
	Final string     `json:"final"`
}

// CaseCall is one tool call the offline model makes, with its raw JSON input.
type CaseCall struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// CaseExpect is what the answer must satisfy: files it must cite and substrings it must contain.
type CaseExpect struct {
	FileIDs        []string `json:"fileIds"`
	AnswerContains []string `json:"answerContains"`
}

// LoadCases reads every golden case, in a stable order.
func LoadCases() ([]Case, error) {
	entries, err := caseFS.ReadDir("cases")
	if err != nil {
		return nil, fmt.Errorf("read cases: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	cases := make([]Case, 0, len(names))
	for _, name := range names {
		data, err := caseFS.ReadFile("cases/" + name)
		if err != nil {
			return nil, fmt.Errorf("read case %q: %w", name, err)
		}
		var c Case
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("decode case %q: %w", name, err)
		}
		cases = append(cases, c)
	}
	return cases, nil
}

// seed loads a case's files into the fake index and retriever, mirroring what ingest does on drop:
// the file is registered and made retrievable.
func seed(idx *fakeIndex, retriever *fakeRetriever, c Case) error {
	ctx := context.Background()
	for _, cf := range c.Files {
		file := domain.File{
			ID:        cf.ID,
			OwnerID:   c.Owner,
			Name:      cf.Name,
			Key:       "files/" + cf.ID,
			Meta:      cf.Meta,
			CreatedAt: caseDate(cf.Meta),
		}
		if err := idx.Put(ctx, file); err != nil {
			return fmt.Errorf("index case file %q: %w", cf.ID, err)
		}
		retriever.add(file)
	}
	return nil
}

// caseDate reads a file's own date from its metadata, so date-range filters have something to
// match. An absent or unparseable date leaves the created time zero. A since bound would then drop
// the file, so a case that filters by since must give its files a date.
func caseDate(meta map[string]string) time.Time {
	parsed, err := time.Parse("2006-01-02", meta["date"])
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

// scriptedModel is the offline model: it replays a case's tool calls, then returns its final
// reply, standing in for the real model so the plumbing and assertions run without Bedrock.
type scriptedModel struct {
	script CaseScript
}

func (m scriptedModel) Converse(ctx context.Context, c llm.Conversation) (string, error) {
	for _, call := range m.script.Calls {
		if _, err := c.Execute(ctx, llm.ToolCall{Name: call.Name, Input: call.Input}); err != nil {
			return "", fmt.Errorf("execute %s: %w", call.Name, err)
		}
	}
	return m.script.Final, nil
}

// noopRecorder drops the LLM call trace, since the eval does not inspect it.
type noopRecorder struct{}

func (noopRecorder) Record(context.Context, llm.Call) {}

// envOr returns the environment variable value, or fallback when it is unset.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
