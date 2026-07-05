package extract

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zipBytes builds an in-memory zip from name to content, standing in for an OOXML file.
func zipBytes(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range parts {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestIsOffice(t *testing.T) {
	assert.True(t, isOffice("application/vnd.openxmlformats-officedocument.wordprocessingml.document"))
	assert.False(t, isOffice("text/plain"))
	assert.False(t, isOffice("image/jpeg"))
}

// officeTextCase is one OOXML content-part fixture and the text it should yield.
type officeTextCase struct {
	name  string
	parts map[string]string
	want  string
}

func TestOfficeTextReadsContentParts(t *testing.T) {
	tests := []officeTextCase{
		{
			name:  "docx",
			parts: map[string]string{"word/document.xml": `<w:body><w:p><w:r><w:t>Hello</w:t></w:r><w:r><w:t>world</w:t></w:r></w:p></w:body>`},
			want:  "Hello",
		},
		{
			name:  "xlsx shared strings",
			parts: map[string]string{"xl/sharedStrings.xml": `<sst><si><t>Total</t></si><si><t>52.30</t></si></sst>`},
			want:  "Total",
		},
		{
			name:  "pptx slide",
			parts: map[string]string{"ppt/slides/slide1.xml": `<p:sld><a:t>Slide text</a:t></p:sld>`},
			want:  "Slide text",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			text, err := officeText(zipBytes(t, tc.parts))

			// Assert
			require.NoError(t, err)
			assert.Contains(t, text, tc.want)
		})
	}
}

func TestOfficeTextSkipsNonContentParts(t *testing.T) {
	// Arrange: only app-level metadata, no document body.
	content := zipBytes(t, map[string]string{"docProps/app.xml": `<Properties><Application>Word</Application></Properties>`})

	// Act
	text, err := officeText(content)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, text)
}

func TestOfficeTextRejectsNonZip(t *testing.T) {
	// Act
	_, err := officeText([]byte("not a zip"))

	// Assert
	assert.Error(t, err)
}

func TestFileBlockOfficeDecodesToText(t *testing.T) {
	// Arrange
	content := zipBytes(t, map[string]string{"word/document.xml": `<w:t>Invoice total 52.30</w:t>`})

	// Act
	block := fileBlock(content, docxType)

	// Assert
	require.NotNil(t, block.OfText)
	assert.Contains(t, block.OfText.Text, "Invoice total")
}

func TestFileBlockOfficeFallsBackWhenEmpty(t *testing.T) {
	// Arrange: an office file with no readable content part.
	content := zipBytes(t, map[string]string{"docProps/app.xml": `<Properties/>`})

	// Act
	block := fileBlock(content, docxType)

	// Assert: a non-empty placeholder so the model still returns valid JSON.
	require.NotNil(t, block.OfText)
	assert.Equal(t, "(no readable text in this document)", block.OfText.Text)
}
