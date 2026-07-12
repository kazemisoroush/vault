package blob

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

// Put writes bytes to a key with the given content type, overwriting any existing object.
func (s *S3Store) Put(ctx context.Context, key string, contentType string, content []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		Body:        bytes.NewReader(content),
	})
	if err != nil {
		return fmt.Errorf("put object %q: %w", key, err)
	}
	return nil
}

// Get reads an object's bytes and content type from the bucket.
func (s *S3Store) Get(ctx context.Context, key string) ([]byte, string, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("get object %q: %w", key, err)
	}
	defer func() { _ = out.Body.Close() }()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read object %q: %w", key, err)
	}

	contentType := ""
	if out.ContentType != nil {
		contentType = *out.ContentType
	}
	return data, contentType, nil
}

// Copy duplicates an object within the bucket, overwriting the destination.
func (s *S3Store) Copy(ctx context.Context, srcKey string, dstKey string) error {
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(s.bucket + "/" + srcKey),
		Key:        aws.String(dstKey),
	})
	if err != nil {
		return fmt.Errorf("copy object %q to %q: %w", srcKey, dstKey, err)
	}

	return nil
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
