package chunk_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kazemisoroush/vault/backend/internal/chunk"
)

func TestChunksKeepsNameFieldsAndBodyAsSeparateChunks(t *testing.T) {
	// Arrange + Act
	got := chunk.Chunks(
		"passport.jpg",
		map[string]string{"passport_number": "RA3495037", "country": "Australia"},
		"line one\n\nline two",
	)

	// Assert: the name, each metadata field, and each body passage are their own chunk.
	assert.Contains(t, got, "passport.jpg")
	assert.Contains(t, got, "passport_number: RA3495037")
	assert.Contains(t, got, "country: Australia")
	assert.Contains(t, got, "line one")
	assert.Contains(t, got, "line two")
}

func TestChunksSplitsALongBodyIntoSeveralPassages(t *testing.T) {
	// Arrange: a body well over the passage budget, one long line.
	body := strings.Repeat("word ", 400)

	// Act
	got := chunk.Chunks("f.txt", nil, body)

	// Assert: the name plus more than one body passage.
	assert.Greater(t, len(got), 2)
	for _, c := range got {
		assert.LessOrEqual(t, len([]rune(c)), 512)
	}
}

func TestChunksWithNoBodyStillHasNameAndFields(t *testing.T) {
	// Arrange + Act: an image with no readable text still yields findable chunks.
	got := chunk.Chunks("IMG_4326.JPG", map[string]string{"document_type": "Passport"}, "")

	// Assert
	assert.Equal(t, []string{"IMG_4326.JPG", "document_type: Passport"}, got)
}

func TestChunksDeduplicatesRepeats(t *testing.T) {
	// Arrange + Act: a name that equals a body line appears once.
	got := chunk.Chunks("hello", nil, "hello")

	// Assert
	assert.Equal(t, []string{"hello"}, got)
}
