package blob

import "strings"

// keyPrefix is the S3 namespace for stored (content-addressed) file blobs; stagingPrefix is where a
// fresh upload lands before it is hashed and moved to its content-addressed key.
const (
	keyPrefix     = "files/"
	stagingPrefix = "uploads/"
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
