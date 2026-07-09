package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
)

// defaultMaxRounds caps how many model calls one Converse may make when MaxRounds is not set.
// It stops a misbehaving model that keeps calling tools from looping forever.
const defaultMaxRounds = 4

// Tool is a function the model may call. Schema is the JSON Schema of the tool's input, given as
// its properties map and the list of required property names.
type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
	Required    []string
}

// ToolCall is one tool invocation the model asked for.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// ToolExecutor runs one tool call and returns the result text handed back to the model. An error
// is reported back to the model as a failed tool result, so the model can react rather than abort.
type ToolExecutor func(ctx context.Context, call ToolCall) (string, error)

// Conversation is one tool-using exchange: a question, the tools the model may call, and the
// function that runs them.
type Conversation struct {
	// System is an optional system prompt.
	System string
	// Prompt is the user's question and the trace label for the first call.
	Prompt string
	// MaxTokens caps each model reply.
	MaxTokens int64
	// Tools the model may call.
	Tools []Tool
	// Execute runs a tool call the model asked for.
	Execute ToolExecutor
	// MaxRounds caps the model calls. Zero or less uses defaultMaxRounds.
	MaxRounds int
}

// Converse runs a tool-using exchange to a final text answer. The model may call the given tools;
// each call is run by Execute and its result is fed back, until the model stops asking for tools
// or the round cap is reached. Every model call is recorded on the trace.
func (m *Model) Converse(ctx context.Context, c Conversation) (string, error) {
	rounds := c.MaxRounds
	if rounds <= 0 {
		rounds = defaultMaxRounds
	}

	tools := toolUnions(c.Tools)
	messages := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(c.Prompt))}

	for round := 0; round < rounds; round++ {
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(m.name),
			MaxTokens: c.MaxTokens,
			Messages:  messages,
			Tools:     tools,
		}
		if c.System != "" {
			params.System = []anthropic.TextBlockParam{{Text: c.System}}
		}

		resp, err := m.send(ctx, roundLabel(c.Prompt, round), params)
		if err != nil {
			return "", err
		}
		if resp.StopReason != anthropic.StopReasonToolUse {
			return collectText(resp.Content), nil
		}

		messages = append(messages, resp.ToParam())
		messages = append(messages, anthropic.NewUserMessage(m.runTools(ctx, resp, c.Execute)...))
	}

	return "", fmt.Errorf("tool loop did not finish within %d rounds", rounds)
}

// runTools runs every tool the model asked for in one reply and returns their results as the
// blocks of the next user turn. A tool error becomes a failed tool result rather than aborting.
func (m *Model) runTools(ctx context.Context, resp *anthropic.Message, execute ToolExecutor) []anthropic.ContentBlockParamUnion {
	var results []anthropic.ContentBlockParamUnion
	for _, block := range resp.Content {
		if block.Type != "tool_use" {
			continue
		}
		output, err := execute(ctx, ToolCall{ID: block.ID, Name: block.Name, Input: block.Input})
		if err != nil {
			results = append(results, anthropic.NewToolResultBlock(block.ID, "error: "+err.Error(), true))
			continue
		}
		results = append(results, anthropic.NewToolResultBlock(block.ID, output, false))
	}
	return results
}

// toolUnions turns the Vault tool definitions into the SDK's tool params.
func toolUnions(tools []Tool) []anthropic.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	unions := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		schema := anthropic.ToolInputSchemaParam{Properties: tool.Schema, Required: tool.Required}
		union := anthropic.ToolUnionParamOfTool(schema, tool.Name)
		if tool.Description != "" {
			union.OfTool.Description = anthropic.String(tool.Description)
		}
		unions = append(unions, union)
	}
	return unions
}

// roundLabel is the trace label for a round: the question on the first call, a note after.
func roundLabel(prompt string, round int) string {
	if round == 0 {
		return prompt
	}
	return fmt.Sprintf("tool results round %d", round)
}
