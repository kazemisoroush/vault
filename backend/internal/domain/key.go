package domain

import "strings"

// keyPrefix is the S3 key namespace for stored file blobs.
const keyPrefix = "files/"

// Key returns the S3 object key for a file id.
func Key(id string) string {
	return keyPrefix + id
}

// IDFromKey returns the file id embedded in an S3 object key.
func IDFromKey(key string) string {
	return strings.TrimPrefix(key, keyPrefix)
}
