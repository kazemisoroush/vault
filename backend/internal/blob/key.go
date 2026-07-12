package blob

import "strings"

// keyPrefix holds content-addressed file blobs, stagingPrefix holds fresh uploads awaiting a
// hash, and textPrefix holds each file's canonical extracted text, the record the check gate
// verifies quoted spans against.
const (
	keyPrefix     = "files/"
	stagingPrefix = "uploads/"
	textPrefix    = "text/"
)

// Key returns the content-addressed S3 object key for a file id (its content hash).
func Key(id string) string {
	return keyPrefix + id
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
