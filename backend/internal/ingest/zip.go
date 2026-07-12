package ingest

import (
	"context"
	"fmt"
	"log"
	"path"

	"github.com/kazemisoroush/vault/backend/internal/archive"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
)

// expand explodes an archive into individual staged uploads, one per inner file, so the normal
// pipeline ingests each on its own (metadata, embedding, and the throttle redrive included). The
// archive itself is never stored as a file. A child upload id is derived from the archive hash and
// the entry path, so a redriven expansion re-stages the same ids rather than duplicating files. An
// archive that yields no files is marked failed rather than vanishing.
func (h *Handler) expand(ctx context.Context, archiveFile domain.File, stagingKey string, content []byte) error {
	zipHash := hashHex(content)
	staged := 0
	var stageErr error
	for file, err := range h.unpacker.Unpack(content) {
		if err != nil {
			// The archive could not be opened, so there is nothing to ingest.
			log.Printf("archive %s could not be opened: %v", archiveFile.ID, err)
			return h.markArchiveFailed(ctx, archiveFile, stagingKey)
		}
		if stageErr = h.stageChild(ctx, archiveFile.OwnerID, zipHash, file); stageErr != nil {
			break
		}
		staged++
	}
	if stageErr != nil {
		// A staging failure fails the invocation so the whole archive is redriven; the
		// deterministic child ids make that safe to repeat.
		return stageErr
	}
	if staged == 0 {
		log.Printf("archive %s held no files to ingest, marking failed", archiveFile.ID)
		return h.markArchiveFailed(ctx, archiveFile, stagingKey)
	}

	log.Printf("archive %s expanded into %d files", archiveFile.ID, staged)
	// The archive is not kept: drop its pending record and staging object.
	h.cleanup(ctx, archiveFile.ID, stagingKey)
	return nil
}

// stageChild writes one inner file as a fresh staged upload with its own pending record, which the
// S3 event then ingests. The record is written before the object so the event finds it.
func (h *Handler) stageChild(ctx context.Context, ownerID string, zipHash string, file archive.File) error {
	childID := childUploadID(zipHash, file.Name)
	key := blob.StagingKey(childID)
	now := h.now().UTC()
	child := domain.File{
		ID:          childID,
		OwnerID:     ownerID,
		Key:         key,
		Name:        path.Base(file.Name),
		ContentType: file.ContentType,
		Size:        int64(len(file.Data)),
		Status:      domain.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.index.Put(ctx, child); err != nil {
		return fmt.Errorf("index child %q: %w", file.Name, err)
	}
	if err := h.blobs.Put(ctx, key, file.ContentType, file.Data); err != nil {
		return fmt.Errorf("stage child %q: %w", file.Name, err)
	}
	return nil
}

// markArchiveFailed records the archive itself as failed and removes its staging object, so an
// oversized, corrupt, or empty archive has a terminal state rather than sitting pending. The record
// is kept and settled to failed, which handleKey treats as a no-op on redelivery.
func (h *Handler) markArchiveFailed(ctx context.Context, archiveFile domain.File, stagingKey string) error {
	if _, err := h.save(ctx, archiveFile, domain.StatusFailed, nil); err != nil {
		return err
	}
	if err := h.blobs.Delete(ctx, stagingKey); err != nil {
		log.Printf("delete staging %s: %v", stagingKey, err)
	}
	return nil
}

// childUploadID derives a stable staging id for one inner file from the archive hash and the entry
// path, so re-expanding an archive re-uses the same ids instead of creating duplicates.
func childUploadID(zipHash string, name string) string {
	return hashHex([]byte(zipHash + "\x00" + name))
}
