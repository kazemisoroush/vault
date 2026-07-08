package knowledge

import (
	"context"
	"fmt"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/embed"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/vectors"
)

// listPageSize is how many records Query pulls from the index per page while it scans an owner.
const listPageSize = int32(100)

// Store is the Base backed by the embedder, the vector store, and the file index that already
// serve the rest of Vault. It adds no storage of its own; it composes what exists.
type Store struct {
	embedder embed.Embedder
	vectors  vectors.Store
	index    index.Index
}

// NewStore builds a Store over the embedder, vector store, and index.
func NewStore(embedder embed.Embedder, store vectors.Store, idx index.Index) *Store {
	return &Store{embedder: embedder, vectors: store, index: idx}
}

// Search embeds the query, pulls the owner's nearest files from the vector store, loads their
// records, and keeps those that also pass the filter. Order stays closest first.
func (s *Store) Search(ctx context.Context, ownerID, query string, filter Filter, topK int) ([]domain.File, error) {
	vector, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	ids, err := s.vectors.Query(ctx, ownerID, vector, int32(topK))
	if err != nil {
		return nil, fmt.Errorf("query vectors: %w", err)
	}

	files := make([]domain.File, 0, len(ids))
	for _, id := range ids {
		file, err := s.index.Get(ctx, id)
		if err != nil {
			continue // a vector without a record is skipped, same as the ask path does today
		}
		if file.OwnerID != ownerID || !filter.matches(file) {
			continue
		}
		files = append(files, file)
	}
	return files, nil
}

// Query returns the owner's files that pass the filter. It scans the owner's records page by
// page and filters them in memory, which is enough at single-user scale and needs no new index.
func (s *Store) Query(ctx context.Context, ownerID string, filter Filter) ([]domain.File, error) {
	matches := make([]domain.File, 0)
	cursor := ""
	for {
		page, next, err := s.index.List(ctx, ownerID, listPageSize, cursor)
		if err != nil {
			return nil, fmt.Errorf("list owner files: %w", err)
		}
		for _, file := range page {
			if filter.matches(file) {
				matches = append(matches, file)
			}
		}
		if next == "" {
			break
		}
		cursor = next
	}
	return matches, nil
}

// Fetch returns one file the owner owns, with the text the knowledge base holds about it. A file
// the caller does not own is reported as not found, so its existence never leaks.
func (s *Store) Fetch(ctx context.Context, ownerID, id string) (Document, error) {
	file, err := s.index.Get(ctx, id)
	if err != nil {
		return Document{}, fmt.Errorf("get file %q: %w", id, err)
	}
	if file.OwnerID != ownerID {
		return Document{}, fmt.Errorf("get file %q: %w", id, index.ErrNotFound)
	}
	return Document{File: file, Text: file.SearchText()}, nil
}
