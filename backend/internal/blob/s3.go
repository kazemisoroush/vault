package blob

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Store is the S3 implementation of Store.
type S3Store struct {
	bucket    string
	client    *s3.Client
	presigner *s3.PresignClient
}

// NewS3Store builds an S3Store for one bucket.
func NewS3Store(client *s3.Client, bucket string) *S3Store {
	return &S3Store{
		bucket:    bucket,
		client:    client,
		presigner: s3.NewPresignClient(client),
	}
}

// PresignPut returns a URL the client can PUT the file bytes to.
func (s *S3Store) PresignPut(ctx context.Context, key string, contentType string, expiry time.Duration) (string, error) {
	req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign put %q: %w", key, err)
	}

	return req.URL, nil
}

// PresignGet returns a URL the client can GET the file bytes from.
func (s *S3Store) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign get %q: %w", key, err)
	}

	return req.URL, nil
}

// Delete removes the object from the bucket.
func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object %q: %w", key, err)
	}

	return nil
}
