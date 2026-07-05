package extract

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const docxType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

func TestEmbeddedMetaOfficeCoreProps(t *testing.T) {
	// Arrange: an office file carrying core properties.
	core := `<cp:coreProperties xmlns:cp="a" xmlns:dc="b" xmlns:dcterms="c">
		<dc:creator>Soroush Kazemi</dc:creator>
		<dc:title>Integration Audit</dc:title>
		<dcterms:created>2026-06-25T09:00:00Z</dcterms:created>
		<dcterms:modified>2026-06-26T11:30:00Z</dcterms:modified>
	</cp:coreProperties>`
	content := zipBytes(t, map[string]string{"docProps/core.xml": core})

	// Act
	meta := embeddedMeta(content, docxType)

	// Assert
	assert.Equal(t, "Soroush Kazemi", meta["author"])
	assert.Equal(t, "Integration Audit", meta["title"])
	assert.Equal(t, "2026-06-25", meta["created"])
	assert.Equal(t, "2026-06-26", meta["modified"])
}

func TestEmbeddedMetaImageWithoutExifIsEmpty(t *testing.T) {
	// Arrange: bytes that are not a valid EXIF image.
	// Act
	meta := embeddedMeta([]byte("not an image"), "image/jpeg")

	// Assert
	assert.Empty(t, meta)
}

func TestEmbeddedMetaUnknownTypeIsEmpty(t *testing.T) {
	// Act + Assert
	assert.Empty(t, embeddedMeta([]byte("plain text"), "text/plain"))
}

func TestDateOnly(t *testing.T) {
	assert.Equal(t, "2026-06-25", dateOnly("2026-06-25T09:00:00Z"))
	assert.Equal(t, "already-a-date", dateOnly("already-a-date"))
	require.Equal(t, "", dateOnly(""))
}
