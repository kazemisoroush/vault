package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

func TestKeyRoundTrip(t *testing.T) {
	// Arrange
	id := "abc-123"

	// Act
	key := domain.Key(id)

	// Assert
	assert.Equal(t, "files/abc-123", key)
	assert.Equal(t, id, domain.IDFromKey(key))
}
