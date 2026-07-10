package llm

import (
	"encoding/base64"

	"github.com/anthropics/anthropic-sdk-go"
)

// Part is one piece of a user turn. Callers build parts with Text, Image, or Document, so the
// Anthropic content types stay inside this package.
type Part interface {
	block() anthropic.ContentBlockParamUnion
}

// Text is a plain text part.
func Text(text string) Part {
	return textPart{text: text}
}

// Image is an image part from its raw bytes and media type, for example "image/jpeg".
func Image(mediaType string, data []byte) Part {
	return imagePart{mediaType: mediaType, data: data}
}

// Document is a PDF part from its raw bytes.
func Document(data []byte) Part {
	return documentPart{data: data}
}

type textPart struct {
	text string
}

func (p textPart) block() anthropic.ContentBlockParamUnion {
	return anthropic.NewTextBlock(p.text)
}

type imagePart struct {
	mediaType string
	data      []byte
}

func (p imagePart) block() anthropic.ContentBlockParamUnion {
	return anthropic.NewImageBlockBase64(p.mediaType, base64.StdEncoding.EncodeToString(p.data))
}

type documentPart struct {
	data []byte
}

func (p documentPart) block() anthropic.ContentBlockParamUnion {
	return anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{Data: base64.StdEncoding.EncodeToString(p.data)})
}

// blocks turns the parts into the SDK content blocks of a user turn.
func blocks(parts []Part) []anthropic.ContentBlockParamUnion {
	out := make([]anthropic.ContentBlockParamUnion, len(parts))
	for i, part := range parts {
		out[i] = part.block()
	}
	return out
}
