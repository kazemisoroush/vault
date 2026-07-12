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

	"github.com/kazemisoroush/vault/backend/internal/archive"
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
	unpacker  archive.Unpacker
	now       func() time.Time
}

// NewHandler builds an ingest Handler with a real clock and the default zip unpacker.
func NewHandler(idx index.Index, blobs blob.Store, extractor extract.Extractor, embedder embed.Embedder, store vectors.Store) *Handler {
	return &Handler{index: idx, blobs: blobs, extractor: extractor, embedder: embedder, vectors: store, unpacker: archive.Zip{}, now: time.Now}
}

// Handle processes every object-created record in an S3 event.
func (h *Handler) Handle(ctx context.Context, event events.S3Event) error {
	for _, record := range event.Records {
		key, err := url.QueryUnescape(record.S3.Object.Key)
		if err != nil {
			return fmt.Errorf("decode key %q: %w", record.S3.Object.Key, err)
		}
		// The size comes from the event, not the client-declared record, so an archive guard
		// cannot be bypassed by under-declaring the upload size.
		if err := h.handleKey(ctx, key, record.S3.Object.Size); err != nil {
			return fmt.Errorf("handle upload %q: %w", key, err)
		}
	}
	return nil
}

// handleKey settles one staged upload under its content hash, which makes a re-drop idempotent.
// objectSize is the real size of the object from the S3 event.
func (h *Handler) handleKey(ctx context.Context, stagingKey string, objectSize int64) error {
	uploadID := blob.IDFromStagingKey(stagingKey)

	pending, err := h.index.Get(ctx, uploadID)
	if errors.Is(err, index.ErrNotFound) {
		return nil // already settled, so a redelivered event is a no-op
	}
	if err != nil {
		return fmt.Errorf("get pending %q: %w", uploadID, err)
	}

	// A record already settled to a terminal state (for example a failed archive whose staging
	// object was removed) is a no-op on redelivery, rather than erroring on the missing object.
	if pending.Status == domain.StatusReady || pending.Status == domain.StatusFailed {
		return nil
	}

	// Refuse an archive larger than the cap before loading it, so a huge zip cannot exhaust memory.
	// The size is the trusted object size from the event, not the client-declared value.
	if archive.IsZipContentType(pending.ContentType) && objectSize > archive.MaxArchiveBytes {
		log.Printf("archive %s is %d bytes, over the %d cap; marking failed", uploadID, objectSize, int64(archive.MaxArchiveBytes))
		if err := h.markArchiveFailed(ctx, pending, stagingKey); err != nil {
			return fmt.Errorf("mark oversized archive %q: %w", uploadID, err)
		}
		return nil
	}

	content, contentType, err := h.blobs.Get(ctx, stagingKey)
	if err != nil {
		return fmt.Errorf("read staged %q: %w", stagingKey, err)
	}

	// A zip archive is unpacked into its inner files rather than stored as one file.
	if archive.IsZip(content, contentType) {
		if err := h.expand(ctx, pending, stagingKey, content); err != nil {
			return fmt.Errorf("expand archive %q: %w", uploadID, err)
		}
		return nil
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
			return fmt.Errorf("record failed extraction for %q: %w", hash, err)
		}
		h.cleanup(ctx, uploadID, stagingKey)
		return nil
	}

	saved, err := h.save(ctx, file, domain.StatusReady, meta)
	if err != nil {
		return fmt.Errorf("settle %q: %w", hash, err)
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
