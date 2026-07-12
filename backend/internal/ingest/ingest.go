// Package ingest fills a dropped file's metadata from an S3 event.
package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// NewHandler builds an ingest Handler with a real clock.
func NewHandler(idx index.Index, blobs blob.Store, extractor extract.Extractor, embedder embed.Embedder, store vectors.Store) *Handler {
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

// handleKey settles one staged upload under its content hash, which makes a re-drop idempotent.
func (h *Handler) handleKey(ctx context.Context, stagingKey string) error {
	uploadID := blob.IDFromStagingKey(stagingKey)

	pending, err := h.index.Get(ctx, uploadID)
	if errors.Is(err, index.ErrNotFound) {
		return nil // already settled, so a redelivered event is a no-op
	}
	if err != nil {
		return fmt.Errorf("get pending %q: %w", uploadID, err)
	}

	content, contentType, err := h.blobs.Get(ctx, stagingKey)
	if err != nil {
		return fmt.Errorf("read staged %q: %w", stagingKey, err)
	}

	hash := hashHex(content)
	file := pending
	file.ID = hash
	file.Key = blob.Key(hash)

	if err := h.blobs.Copy(ctx, stagingKey, file.Key); err != nil {
		return fmt.Errorf("copy to %q: %w", file.Key, err)
	}

	meta, err := h.extractor.Extract(ctx, content, contentType)
	if err != nil {
		if errors.Is(err, extract.ErrRetryable) {
			// Extraction is throttled or briefly unavailable. Leave the pending record and
			// staging object untouched and fail the invocation, so the S3 event is redriven
			// later instead of losing the file to a terminal failed state.
			log.Printf("extraction throttled for %s, will retry: %v", hash, err)
			return fmt.Errorf("extract %s: %w", hash, err)
		}
		log.Printf("extraction failed for %s: %v", hash, err)
		if _, err := h.save(ctx, file, domain.StatusFailed, nil); err != nil {
			return err
		}
		h.cleanup(ctx, uploadID, stagingKey)
		return nil
	}

	saved, err := h.save(ctx, file, domain.StatusReady, meta)
	if err != nil {
		return err
	}
	h.embed(ctx, saved)
	h.cleanup(ctx, uploadID, stagingKey)
	return nil
}

// cleanup removes the settled file's staging record and object, logging failures rather than failing.
func (h *Handler) cleanup(ctx context.Context, uploadID string, stagingKey string) {
	if err := h.index.Delete(ctx, uploadID); err != nil {
		log.Printf("delete pending %s: %v", uploadID, err)
	}
	if err := h.blobs.Delete(ctx, stagingKey); err != nil {
		log.Printf("delete staging %s: %v", stagingKey, err)
	}
}

// hashHex returns the SHA-256 of the content as hex, the id a file is stored under.
func hashHex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// embed stores the vector for a ready file so it can be found by meaning. A failure here is
// logged, not fatal: the record is already saved and can be re-embedded later.
func (h *Handler) embed(ctx context.Context, file domain.File) {
	vector, err := h.embedder.Embed(ctx, file.SearchText())
	if err != nil {
		log.Printf("embed %s: %v", file.ID, err)
		return
	}
	if err := h.vectors.Put(ctx, file.ID, file.OwnerID, vector); err != nil {
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
