package retrieve

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

func TestBuildCatalog(t *testing.T) {
	// Arrange
	files := []domain.File{{
		ID:        "abc",
		Name:      "receipt.jpg",
		Meta:      map[string]string{"vendor": "Shell"},
		CreatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}}

	// Act
	got, err := buildCatalog(files)

	// Assert
	require.NoError(t, err)
	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "abc", entries[0]["id"])
	require.Equal(t, "receipt.jpg", entries[0]["name"])
	require.Equal(t, "2026-06-01T00:00:00Z", entries[0]["createdAt"])
}
