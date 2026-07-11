package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

func TestRenderAnswerPromptListsEveryTool(t *testing.T) {
	// Act
	prompt, err := renderAnswerPrompt(answerPromptTemplate, tools())

	// Assert: the placeholder is filled and every tool the agent offers is named in the prompt.
	require.NoError(t, err)
	assert.NotContains(t, prompt, "{{")
	for _, tool := range tools() {
		assert.Contains(t, prompt, tool.Name)
	}
	assert.Contains(t, prompt, `"fileIds"`)
}

func TestRenderAnswerPromptRejectsAnEmptyTemplate(t *testing.T) {
	// Act
	_, err := renderAnswerPrompt("   ", tools())

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestRenderAnswerPromptRejectsAnUnfilledPlaceholder(t *testing.T) {
	// Arrange: a template with a placeholder the renderer does not fill.
	template := "# Role\n{{ TOOLS }}\n{{ SOMETHING_ELSE }}"

	// Act
	_, err := renderAnswerPrompt(template, []llm.Tool{{Name: "x", Description: "y"}})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unfilled placeholder")
}
