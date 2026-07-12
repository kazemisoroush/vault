package archive

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"mime"
	"path"
	"strings"
)

// These bounds cap how much one archive can produce, so a crafted file cannot exhaust the process.
// MaxArchiveBytes is exported because the caller checks it before loading the archive at all.
const (
	MaxArchiveBytes = 512 << 20 // largest archive to load
	maxTotalBytes   = 1 << 30   // total bytes pulled from one archive
	maxEntries      = 512       // files pulled from one archive
	maxEntryBytes   = 64 << 20  // per inner file
)

// zipContentTypes are the content types a plain archive arrives as. Office documents are also zips
// but arrive with their own OOXML content type, so they never match and are left whole.
var zipContentTypes = map[string]bool{
	"application/zip":              true,
	"application/x-zip-compressed": true,
}

// IsZipContentType reports whether a content type is a plain zip archive.
func IsZipContentType(contentType string) bool {
	return zipContentTypes[contentType]
}

// IsZip reports whether the bytes are a zip archive to unpack: a zip content type backed by the zip
// magic number, so a mislabelled file is not torn apart.
func IsZip(content []byte, contentType string) bool {
	return IsZipContentType(contentType) && hasZipMagic(content)
}

// hasZipMagic reports whether content starts with the local file header signature PK\x03\x04.
func hasZipMagic(content []byte) bool {
	return len(content) >= 4 && content[0] == 'P' && content[1] == 'K' && content[2] == 0x03 && content[3] == 0x04
}

// Zip unpacks a zip archive. It skips directories, empty entries, archiver bookkeeping (__MACOSX,
// .DS_Store, ._ forks), and nested archives (one level only), and bounds the work with entry,
// per-file, and total-size caps measured on the bytes actually read.
type Zip struct{}

// Unpack returns the real files inside a zip. An unreadable entry is skipped rather than failing the
// whole archive. Caps stop early rather than erroring, so a partial archive still yields its files.
func (Zip) Unpack(content []byte) ([]File, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	var files []File
	var total int64
	for _, entry := range reader.File {
		if len(files) >= maxEntries {
			break
		}
		if skipEntry(entry) || isNestedZip(entry.Name) {
			continue
		}
		data, err := readEntry(entry)
		if err != nil {
			continue
		}
		total += int64(len(data))
		if total > maxTotalBytes {
			break
		}
		files = append(files, File{
			Name:        entry.Name,
			ContentType: contentTypeForName(entry.Name),
			Data:        data,
		})
	}
	return files, nil
}

// skipEntry reports whether an entry is not a real file to ingest: a directory, an empty file, or
// archiver bookkeeping such as __MACOSX and .DS_Store.
func skipEntry(entry *zip.File) bool {
	if entry.FileInfo().IsDir() || entry.UncompressedSize64 == 0 {
		return true
	}
	base := path.Base(entry.Name)
	return strings.HasPrefix(entry.Name, "__MACOSX/") || base == ".DS_Store" || strings.HasPrefix(base, "._")
}

// isNestedZip reports whether an entry is itself a zip archive, by name. Nested archives are left
// alone, so a zip of zips cannot drive unbounded expansion.
func isNestedZip(name string) bool {
	return strings.EqualFold(path.Ext(name), ".zip")
}

// readEntry reads one entry's bytes, capped so a single inner file cannot exhaust memory regardless
// of its declared uncompressed size.
func readEntry(entry *zip.File) ([]byte, error) {
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
