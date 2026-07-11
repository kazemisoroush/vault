package agent

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// answerPromptTemplate is the answer prompt, kept in its own file so it can be read and edited on
// its own. The {{ TOOLS }} placeholder is filled from the declared tools when the agent starts.
//
//go:embed prompts/answer.prompt
var answerPromptTemplate string

// systemPrompt is the rendered answer prompt, built once at startup from the template and the
// declared tools. A malformed template panics here, since that is a programming error, not input.
var systemPrompt = mustRenderAnswerPrompt(answerPromptTemplate, tools())

// mustRenderAnswerPrompt renders the answer prompt or panics. It exists so the package fails fast
// on a broken template, the same way regexp.MustCompile does.
func mustRenderAnswerPrompt(template string, declared []llm.Tool) string {
	prompt, err := renderAnswerPrompt(template, declared)
	if err != nil {
		panic(fmt.Sprintf("agent: render answer prompt: %v", err))
	}
	return prompt
}

// renderAnswerPrompt fills the answer prompt template with the tool list, so the prompt always
// lists exactly the tools the agent offers. It guards the template: an empty template, or a
// placeholder left unfilled, is an error rather than a silently wrong prompt.
func renderAnswerPrompt(template string, declared []llm.Tool) (string, error) {
	if strings.TrimSpace(template) == "" {
		return "", fmt.Errorf("answer prompt template is empty")
	}

	lines := make([]string, 0, len(declared))
	for _, tool := range declared {
		lines = append(lines, "- "+tool.Name+": "+tool.Description)
	}
	rendered := strings.ReplaceAll(template, "{{ TOOLS }}", strings.Join(lines, "\n"))

	if strings.Contains(rendered, "{{") {
		return "", fmt.Errorf("answer prompt has an unfilled placeholder")
	}
	return rendered, nil
}
