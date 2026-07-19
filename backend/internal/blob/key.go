package blob

import "strings"

// keyPrefix holds content-addressed file blobs, stagingPrefix holds fresh uploads awaiting a
// hash, and textPrefix holds each file's canonical extracted text, the record the check gate
// verifies quoted spans against.
const (
	keyPrefix     = "files/"
	stagingPrefix = "uploads/"
	textPrefix    = "text/"
	kbPrefix      = "kb/"
)

// Key returns the content-addressed S3 object key for a file id (its content hash). This holds the
// raw bytes for download; the Knowledge Base indexes the KB source under kbPrefix instead.
func Key(id string) string {
	return keyPrefix + id
}

// KBKey returns the S3 object key of a file's Knowledge Base source: the searchable representation
// the data source indexes, which is extracted text for an image or PDF, or a copy of a parseable
// document. It is separate from the raw object so the data source never sees a format it rejects.
func KBKey(id string) string {
	return kbPrefix + id
}

// KBMetadataKey returns the metadata sidecar key for a file's Knowledge Base source, which the data
// source reads to attach the file's id and name to every passage it indexes.
func KBMetadataKey(id string) string {
	return KBKey(id) + ".metadata.json"
}

// IDFromKey returns the file id embedded in a content-addressed object key.
func IDFromKey(key string) string {
	return strings.TrimPrefix(key, keyPrefix)
}

// StagingKey returns the S3 object key a fresh upload lands on before it is hashed.
func StagingKey(uploadID string) string {
	return stagingPrefix + uploadID
}

// IDFromStagingKey returns the upload id embedded in a staging object key.
func IDFromStagingKey(key string) string {
	return strings.TrimPrefix(key, stagingPrefix)
}

// TextKey returns the S3 object key of a file's canonical extracted text.
func TextKey(id string) string {
	return textPrefix + id
}
