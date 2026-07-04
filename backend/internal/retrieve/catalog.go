package retrieve

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

// catalogEntry is the compact view of a file the model matches against.
type catalogEntry struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Meta      map[string]string `json:"meta,omitempty"`
	CreatedAt string            `json:"createdAt"`
}

// buildCatalog renders the files as a compact JSON array for the model prompt.
func buildCatalog(files []domain.File) (string, error) {
	entries := make([]catalogEntry, 0, len(files))
	for _, file := range files {
		entries = append(entries, catalogEntry{
			ID:        file.ID,
			Name:      file.Name,
			Meta:      file.Meta,
			CreatedAt: file.CreatedAt.Format(time.RFC3339),
		})
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("marshal catalog: %w", err)
	}
	return string(data), nil
}
