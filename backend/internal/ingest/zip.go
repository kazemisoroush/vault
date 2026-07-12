package ingest

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"path"
	"strings"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/domain"
)

// Zip limits bound how much work one archive can create, so a crafted file cannot exhaust the
// function. maxZipBytes is checked before the archive is loaded, from the record's declared size.
const (
	maxZipBytes      = 512 << 20 // refuse to load an archive larger than this
	maxZipTotalBytes = 1 << 30   // total uncompressed bytes pulled from one archive
	maxZipEntries    = 512       // number of files pulled from one archive
	maxEntryBytes    = 64 << 20  // per inner file
)

// zipContentTypes are the content types a plain archive arrives as. Office documents are also zips
// but arrive with their own OOXML content type, so they never match here and are extracted whole.
var zipContentTypes = map[string]bool{
	"application/zip":              true,
	"application/x-zip-compressed": true,
}

// isZipContentType reports whether a content type is a plain zip archive.
func isZipContentType(contentType string) bool {
	return zipContentTypes[contentType]
}

// isZipArchive reports whether the bytes are a zip archive to unpack: a zip content type backed by
// the zip magic number, so a mislabelled file is not torn apart.
func isZipArchive(content []byte, contentType string) bool {
	return isZipContentType(contentType) && hasZipMagic(content)
}

// hasZipMagic reports whether content starts with the local file header signature PK\x03\x04.
func hasZipMagic(content []byte) bool {
	return len(content) >= 4 && content[0] == 'P' && content[1] == 'K' && content[2] == 0x03 && content[3] == 0x04
}

// expand explodes a zip archive into individual staged uploads, one per inner file, so the normal
// pipeline ingests each on its own (metadata, embedding, and the throttle redrive included). The
// archive itself is never stored as a file. A child upload id is derived from the archive hash and
// the entry name, so a redriven expansion re-stages the same ids rather than duplicating files.
func (h *Handler) expand(ctx context.Context, archive domain.File, stagingKey string, content []byte) error {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		log.Printf("zip %s could not be opened: %v", archive.ID, err)
		return h.markArchiveFailed(ctx, archive, stagingKey)
	}

	zipHash := hashHex(content)
	var total uint64
	staged := 0
	for _, entry := range reader.File {
		if skipZipEntry(entry) {
			continue
		}
		if isNestedZip(entry.Name) {
			// One level only: a nested archive is left alone rather than expanded, so a zip of
			// zips cannot drive unbounded work.
			log.Printf("zip %s: skipping nested archive %q", archive.ID, entry.Name)
			continue
		}
		if staged >= maxZipEntries {
			log.Printf("zip %s: entry cap %d reached, remaining entries skipped", archive.ID, maxZipEntries)
			break
		}
		total += entry.UncompressedSize64
		if total > maxZipTotalBytes {
			log.Printf("zip %s: uncompressed size cap reached, remaining entries skipped", archive.ID)
			break
		}
		data, err := readZipEntry(entry)
		if err != nil {
			log.Printf("zip %s: skipping unreadable entry %q: %v", archive.ID, entry.Name, err)
			continue
		}
		if err := h.stageChild(ctx, archive.OwnerID, zipHash, entry.Name, data); err != nil {
			// A staging failure fails the invocation so the whole archive is redriven; the
			// deterministic child ids make that safe to repeat.
			return err
		}
		staged++
	}

	log.Printf("zip %s expanded into %d files", archive.ID, staged)
	// The archive is not kept: drop its pending record and staging object.
	h.cleanup(ctx, archive.ID, stagingKey)
	return nil
}

// stageChild writes one inner file as a fresh staged upload with its own pending record, which the
// S3 event then ingests. The record is written before the object so the event finds it.
func (h *Handler) stageChild(ctx context.Context, ownerID string, zipHash string, name string, data []byte) error {
	childID := childUploadID(zipHash, name)
	key := blob.StagingKey(childID)
	contentType := contentTypeForName(name)
	now := h.now().UTC()
	child := domain.File{
		ID:          childID,
		OwnerID:     ownerID,
		Key:         key,
		Name:        path.Base(name),
		ContentType: contentType,
		Size:        int64(len(data)),
		Status:      domain.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.index.Put(ctx, child); err != nil {
		return fmt.Errorf("index child %q: %w", name, err)
	}
	if err := h.blobs.Put(ctx, key, contentType, data); err != nil {
		return fmt.Errorf("stage child %q: %w", name, err)
	}
	return nil
}

// markArchiveFailed records the archive itself as failed and removes its staging object, so an
// oversized or corrupt zip has a terminal state rather than sitting pending. The record is kept.
func (h *Handler) markArchiveFailed(ctx context.Context, archive domain.File, stagingKey string) error {
	if _, err := h.save(ctx, archive, domain.StatusFailed, nil); err != nil {
		return err
	}
	if err := h.blobs.Delete(ctx, stagingKey); err != nil {
		log.Printf("delete staging %s: %v", stagingKey, err)
	}
	return nil
}

// childUploadID derives a stable staging id for one inner file, so re-expanding an archive re-uses
// the same ids instead of creating duplicates.
func childUploadID(zipHash string, name string) string {
	return hashHex([]byte(zipHash + "\x00" + name))
}

// skipZipEntry reports whether an entry is not a real file to ingest: a directory, an empty file,
// or archiver bookkeeping such as __MACOSX and .DS_Store.
func skipZipEntry(entry *zip.File) bool {
	if entry.FileInfo().IsDir() || entry.UncompressedSize64 == 0 {
		return true
	}
	base := path.Base(entry.Name)
	return strings.HasPrefix(entry.Name, "__MACOSX/") || base == ".DS_Store" || strings.HasPrefix(base, "._")
}

// isNestedZip reports whether an entry is itself a zip archive, by name.
func isNestedZip(name string) bool {
	return strings.EqualFold(path.Ext(name), ".zip")
}

// readZipEntry reads one entry's bytes, capped so a single inner file cannot exhaust memory.
func readZipEntry(entry *zip.File) ([]byte, error) {
	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("open entry: %w", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(io.LimitReader(rc, maxEntryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read entry: %w", err)
	}
	if int64(len(data)) > maxEntryBytes {
		return nil, fmt.Errorf("entry exceeds %d bytes", int64(maxEntryBytes))
	}
	return data, nil
}

// contentTypeForName guesses an inner file's content type from its extension, so the extractor sees
// the real type (image, pdf) instead of a generic archive blob.
func contentTypeForName(name string) string {
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}
