// Package blob stores file bytes and hands out presigned URLs.
package blob

import (
	"context"
	"time"
)

//go:generate go tool mockgen -source=blob.go -destination=../mocks/blob_mock.go -package=mocks

// Store presigns uploads and downloads, reads, copies, and deletes stored objects.
type Store interface {
	PresignPut(ctx context.Context, key string, contentType string, expiry time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
	Get(ctx context.Context, key string) ([]byte, string, error)
	Copy(ctx context.Context, srcKey string, dstKey string) error
	Delete(ctx context.Context, key string) error
}
