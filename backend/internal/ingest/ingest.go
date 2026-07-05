// Package ingest fills a dropped file's metadata from an S3 event.
package ingest

import (
	"context"
	"fmt"
	"log"
	"maps"
	"net/url"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/embed"
	"github.com/kazemisoroush/vault/backend/internal/extract"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/vectors"
)

// Handler fills a file's metadata after it lands in S3.
type Handler struct {
	index     index.Index
	blobs     blob.Store
	extractor extract.Extractor
	embedder  embed.Embedder
	vectors   vectors.Store
	now       func() time.Time
}

// New builds an ingest Handler with a real clock.
func New(idx index.Index, blobs blob.Store, extractor extract.Extractor, embedder embed.Embedder, store vectors.Store) *Handler {
	return &Handler{index: idx, blobs: blobs, extractor: extractor, embedder: embedder, vectors: store, now: time.Now}
}

// Handle processes every object-created record in an S3 event.
func (h *Handler) Handle(ctx context.Context, event events.S3Event) error {
	for _, record := range event.Records {
		key, err := url.QueryUnescape(record.S3.Object.Key)
		if err != nil {
			return fmt.Errorf("decode key %q: %w", record.S3.Object.Key, err)
		}
		if err := h.handleKey(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// handleKey extracts metadata for one object and updates its record.
func (h *Handler) handleKey(ctx context.Context, key string) error {
	id := blob.IDFromKey(key)

	file, err := h.index.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("get record %q: %w", id, err)
	}

	content, contentType, err := h.blobs.Get(ctx, file.Key)
	if err != nil {
		return fmt.Errorf("read bytes %q: %w", file.Key, err)
	}

	meta, err := h.extractor.Extract(ctx, content, contentType)
	if err != nil {
		log.Printf("extraction failed for %s: %v", id, err)
		_, err := h.save(ctx, file, domain.StatusFailed, nil)
		return err
	}

	saved, err := h.save(ctx, file, domain.StatusReady, meta)
	if err != nil {
		return err
	}

	h.embed(ctx, saved)
	return nil
}

// embed stores the vector for a ready file so it can be found by meaning. A failure here is
// logged, not fatal: the record is already saved and can be re-embedded later.
func (h *Handler) embed(ctx context.Context, file domain.File) {
	vector, err := h.embedder.Embed(ctx, file.SearchText())
	if err != nil {
		log.Printf("embed %s: %v", file.ID, err)
		return
	}
	if err := h.vectors.Put(ctx, file.ID, vector); err != nil {
		log.Printf("store vector for %s: %v", file.ID, err)
	}
}

// save merges any extracted metadata, sets the status, persists the record, and returns it.
func (h *Handler) save(ctx context.Context, file domain.File, status string, meta map[string]string) (domain.File, error) {
	if meta != nil && file.Meta == nil {
		file.Meta = map[string]string{}
	}
	maps.Copy(file.Meta, meta)
	file.Status = status
	file.UpdatedAt = h.now().UTC()

	if err := h.index.Put(ctx, file); err != nil {
		return domain.File{}, fmt.Errorf("save record %q: %w", file.ID, err)
	}
	return file, nil
}
