package checks

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The splitter is the coverage guarantee: every claim must sit at its exact offsets, and no
// checkable sentence may vanish. These tests exercise the boundaries legal text actually has.

func TestSplitBreaksOnSentenceEnders(t *testing.T) {
	// Arrange
	text := "The deposit was paid. Was it on time? It was not!"

	// Act
	claims := split(text)

	// Assert
	require.Len(t, claims, 3)
	assert.Equal(t, "The deposit was paid.", claims[0].Text)
	assert.Equal(t, "Was it on time?", claims[1].Text)
	assert.Equal(t, "It was not!", claims[2].Text)
}

func TestSplitOffsetsReproduceTheText(t *testing.T) {
	// Arrange: offsets are the coverage contract; slicing the input at them must reproduce
	// each claim exactly, or highlighting and later verification would drift.
	text := "Mr. Rossi signed cl. 7.2 on 14 February 2023.\n\nThe deposit of $40,000.00 was paid late. See Rossi v. Meridian."

	// Act
	claims := split(text)

	// Assert
	require.NotEmpty(t, claims)
	for _, claim := range claims {
		assert.Equal(t, claim.Text, text[claim.Start:claim.End], "claim offsets must slice back to the claim")
	}
}

func TestSplitDoesNotBreakOnAbbreviationsInitialsOrNumbers(t *testing.T) {
	// Arrange: dots that are not sentence boundaries in legal prose.
	text := "Mr. J. Rossi relied on cl. 7.2 and s. 12 in Rossi v. Meridian. The amount was $40,000.00 approx. as agreed."

	// Act
	claims := split(text)

	// Assert: two sentences, not a confetti of fragments.
	require.Len(t, claims, 2)
	assert.True(t, strings.HasPrefix(claims[0].Text, "Mr. J. Rossi"))
	assert.True(t, strings.HasPrefix(claims[1].Text, "The amount"))
}

func TestSplitParagraphBreakClosesUnpunctuatedLines(t *testing.T) {
	// Arrange: headings and list items rarely end with punctuation but still get a verdict.
	text := "Chronology of events\nThe contract was executed on 14 February 2023."

	// Act
	claims := split(text)

	// Assert
	require.Len(t, claims, 2)
	assert.Equal(t, "Chronology of events", claims[0].Text)
}

func TestSplitSkipsSegmentsWithNothingToCheck(t *testing.T) {
	// Arrange: stray punctuation and rules assert nothing.
	text := "---\n...\nThe deposit was paid.\n***"

	// Act
	claims := split(text)

	// Assert
	require.Len(t, claims, 1)
	assert.Equal(t, "The deposit was paid.", claims[0].Text)
}

func TestSplitKeepsAttachedClosersWithTheSentence(t *testing.T) {
	// Arrange: a quote or bracket after the full stop belongs to the sentence it closes.
	text := `He wrote "the funds cleared on 2 March." Then he resigned.`

	// Act
	claims := split(text)

	// Assert
	require.Len(t, claims, 2)
	assert.Equal(t, `He wrote "the funds cleared on 2 March."`, claims[0].Text)
	assert.Equal(t, "Then he resigned.", claims[1].Text)
}

func TestSplitEllipsisIsOneBoundary(t *testing.T) {
	// Arrange
	text := "He hesitated... Then he signed."

	// Act
	claims := split(text)

	// Assert
	require.Len(t, claims, 2)
	assert.Equal(t, "He hesitated...", claims[0].Text)
}
