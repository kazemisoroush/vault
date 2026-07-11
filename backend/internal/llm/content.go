package llm

import (
	"github.com/anthropics/anthropic-sdk-go"
)

// Part is one piece of a user turn. Callers build parts with Text, Image, or Document, so the
// Anthropic content types stay inside this package.
type Part interface {
	block() anthropic.ContentBlockParamUnion
}

// blocks turns the parts into the SDK content blocks of a user turn.
func blocks(parts []Part) []anthropic.ContentBlockParamUnion {
	out := make([]anthropic.ContentBlockParamUnion, len(parts))
	for i, part := range parts {
		out[i] = part.block()
	}
	return out
}
