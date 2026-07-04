// Package ingest fills a dropped file's metadata from an S3 event.
package ingest

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/extract"
	"github.com/kazemisoroush/vault/backend/internal/index"
)

// Handler fills a file's metadata after it lands in S3.
type Handler struct {
	index     index.Index
	blobs     blob.Store
	extractor extract.Extractor
	now       func() time.Time
}

// New builds an ingest Handler with a real clock.
func New(idx index.Index, blobs blob.Store, extractor extract.Extractor) *Handler {
	return &Handler{index: idx, blobs: blobs, extractor: extractor, now: time.Now}
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
		return h.save(ctx, file, domain.StatusFailed, nil)
	}

	return h.save(ctx, file, domain.StatusReady, meta)
}

// save merges any extracted metadata, sets the status, and persists the record.
func (h *Handler) save(ctx context.Context, file domain.File, status string, meta map[string]string) error {
	if meta != nil && file.Meta == nil {
		file.Meta = map[string]string{}
	}
	for key, value := range meta {
		file.Meta[key] = value
	}
	file.Status = status
	file.UpdatedAt = h.now().UTC()

	if err := h.index.Put(ctx, file); err != nil {
		return fmt.Errorf("save record %q: %w", file.ID, err)
	}
	return nil
}
