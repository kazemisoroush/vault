package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

func TestSearchTextJoinsNameAndSortedMeta(t *testing.T) {
	// Arrange
	file := domain.File{Name: "petrol.txt", Meta: map[string]string{"vendor": "Shell", "amount": "52.30"}}

	// Act
	got := file.SearchText()

	// Assert: keys are sorted so the text is stable.
	assert.Equal(t, "petrol.txt\namount: 52.30\nvendor: Shell", got)
}

func TestSearchTextWithNoMetaIsJustTheName(t *testing.T) {
	// Arrange
	file := domain.File{Name: "lease.pdf"}

	// Act + Assert
	assert.Equal(t, "lease.pdf", file.SearchText())
}

func TestAttributesFromMetaPicksNormalisedKeys(t *testing.T) {
	// Arrange
	meta := map[string]string{
		"vendor":        "Shell",
		"amount":        "52.30",
		"date":          "2026-02-25",
		"person":        "Soroush",
		"document type": "receipt",
	}

	// Act
	got := domain.AttributesFromMeta(meta)

	// Assert
	assert.Equal(t, domain.Attributes{
		Person:  "Soroush",
		DocType: "receipt",
		Vendor:  "Shell",
		Amount:  "52.30",
		Date:    "2026-02-25",
	}, got)
}

func TestAttributesFromMetaFallsBackToSynonymKeys(t *testing.T) {
	// Arrange: the file uses synonym keys and odd casing, not the canonical ones.
	meta := map[string]string{
		"Merchant": "Woolworths",
		"Total":    "18.00",
		"Type":     "grocery receipt",
	}

	// Act
	got := domain.AttributesFromMeta(meta)

	// Assert: synonyms map onto the normalised attributes, matched without case sensitivity.
	assert.Equal(t, "Woolworths", got.Vendor)
	assert.Equal(t, "18.00", got.Amount)
	assert.Equal(t, "grocery receipt", got.DocType)
	assert.Empty(t, got.Person)
}

func TestAttributesFromMetaEmptyWhenNothingMatches(t *testing.T) {
	// Arrange
	meta := map[string]string{"colour": "blue"}

	// Act + Assert
	assert.Equal(t, domain.Attributes{}, domain.AttributesFromMeta(meta))
}

func TestAttributesFromMetaIsDeterministicOnColludingKeys(t *testing.T) {
	// Arrange: keys that fold to the same lookup key, including an empty value.
	meta := map[string]string{"Vendor": "", "vendor ": "Shell", " VENDOR": "Coles"}

	// Act + Assert: the empty value is never chosen, and the result is stable across runs
	// despite Go's random map iteration order.
	first := domain.AttributesFromMeta(meta).Vendor
	assert.NotEmpty(t, first)
	for range 50 {
		assert.Equal(t, first, domain.AttributesFromMeta(meta).Vendor)
	}
}
