package blob_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kazemisoroush/vault/backend/internal/blob"
)

func TestKeyRoundTrip(t *testing.T) {
	// Arrange
	id := "abc-123"

	// Act
	key := blob.Key(id)

	// Assert
	assert.Equal(t, "files/abc-123", key)
	assert.Equal(t, id, blob.IDFromKey(key))
}
