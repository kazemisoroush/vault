package evals

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazemisoroush/vault/backend/internal/agent"
	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// runCase seeds the case's vault, answers its query with the given model, and checks the answer
// cites the expected files and contains the expected text.
func runCase(t *testing.T, model agent.Converser, c Case) {
	t.Helper()
	retriever := &fakeRetriever{}
	idx := newFakeIndex()
	require.NoError(t, seed(idx, retriever, c))

	answerer := agent.NewAgent(model, retriever, idx)
	result, err := answerer.Answer(context.Background(), c.Owner, c.Query)
	require.NoError(t, err)

	cited := make(map[string]bool, len(result.Files))
	for _, file := range result.Files {
		cited[file.ID] = true
	}
	for _, want := range c.Expect.FileIDs {
		assert.Truef(t, cited[want], "expected file %q among the cited files", want)
	}
	for _, want := range c.Expect.AnswerContains {
		assert.Containsf(t, result.Text, want, "answer should contain %q", want)
	}
}

// TestAgentEvalOffline runs every golden case against the scripted model, so the pipeline and the
// assertions run deterministically in CI without touching Bedrock.
func TestAgentEvalOffline(t *testing.T) {
	cases, err := LoadCases()
	require.NoError(t, err)
	require.NotEmpty(t, cases)

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			runCase(t, scriptedModel{script: c.Script}, c)
		})
	}
}

// TestAgentEvalBedrock runs the same golden cases against the real model on Bedrock. It is gated
// behind VAULT_EVAL_BEDROCK because it needs credentials and spends tokens, so it never runs in CI.
func TestAgentEvalBedrock(t *testing.T) {
	if os.Getenv(envEvalBedrock) == "" {
		t.Skip("set " + envEvalBedrock + "=1 to run the agent eval against Bedrock")
	}
	model := llm.NewModel(
		envOr(envEvalRegion, defaultEvalRegion),
		envOr(envEvalModel, defaultEvalModel),
		evalModelOp,
		noopRecorder{},
	)

	cases, err := LoadCases()
	require.NoError(t, err)
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			runCase(t, model, c)
		})
	}
}
