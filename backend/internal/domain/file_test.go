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
