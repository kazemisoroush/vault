package checks

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

func TestDistinctiveTokensKeepsIdentifiersAndSkipsWordsAndShortNumbers(t *testing.T) {
	// Arrange + Act
	got := distinctiveTokens("Soroush Kazemi's Australian passport number is RA3495037, issued 2024.")

	// Assert: the long identifier is kept; words, a year, and the possessive are dropped.
	assert.Equal(t, []string{"ra3495037"}, got)
}

func TestDistinctiveTokensIsEmptyForAnOrdinarySentence(t *testing.T) {
	// Arrange + Act + Assert: no identifier means no literal scan, so an ordinary claim pays nothing.
	assert.Empty(t, distinctiveTokens("The parties agreed to waive the penalty clause."))
}

func TestRecordContainsTokenMatchesNameOrMetadataCaseInsensitively(t *testing.T) {
	// Arrange
	file := domain.File{Name: "IMG_4326.JPG", Meta: map[string]string{"passport_number": "RA3495037"}}

	// Act + Assert: the token matches the metadata value regardless of case.
	assert.True(t, recordContainsToken(file, []string{"ra3495037"}))
	assert.False(t, recordContainsToken(file, []string{"zz0000000"}))
}
