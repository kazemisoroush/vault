package search

import (
	"context"
	"fmt"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/embed"
	"github.com/kazemisoroush/vault/backend/internal/vectors"
)

// VectorIndexer embeds a file's search text and stores the vector, keyed by file id.
type VectorIndexer struct {
	embedder embed.Embedder
	vectors  vectors.Store
}

// NewVectorIndexer builds a VectorIndexer over an embedder and a vector store.
func NewVectorIndexer(embedder embed.Embedder, store vectors.Store) *VectorIndexer {
	return &VectorIndexer{embedder: embedder, vectors: store}
}

// Index embeds the file's search text and writes its vector.
func (v *VectorIndexer) Index(ctx context.Context, file domain.File) error {
	vector, err := v.embedder.Embed(ctx, file.SearchText())
	if err != nil {
		return fmt.Errorf("embed %s: %w", file.ID, err)
	}
	if err := v.vectors.Put(ctx, file.ID, vector); err != nil {
		return fmt.Errorf("store vector for %s: %w", file.ID, err)
	}
	return nil
}

// Remove deletes a file's vector from the index.
func (v *VectorIndexer) Remove(ctx context.Context, id string) error {
	if err := v.vectors.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete vector for %s: %w", id, err)
	}
	return nil
}
