package vectors

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors/document"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors/types"
)

// ownerKey is the vector metadata field that scopes a query to its owner.
const ownerKey = "owner"

// client is the slice of the S3 Vectors client used, kept small to fake in tests.
type client interface {
	PutVectors(ctx context.Context, in *s3vectors.PutVectorsInput, opts ...func(*s3vectors.Options)) (*s3vectors.PutVectorsOutput, error)
	QueryVectors(ctx context.Context, in *s3vectors.QueryVectorsInput, opts ...func(*s3vectors.Options)) (*s3vectors.QueryVectorsOutput, error)
	DeleteVectors(ctx context.Context, in *s3vectors.DeleteVectorsInput, opts ...func(*s3vectors.Options)) (*s3vectors.DeleteVectorsOutput, error)
}

// S3Vectors is a Store backed by an Amazon S3 Vectors index.
type S3Vectors struct {
	client client
	bucket string
	index  string
}

// NewS3Vectors builds an S3Vectors for a region, vector bucket, and index.
func NewS3Vectors(ctx context.Context, region, bucket, index string) (*S3Vectors, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &S3Vectors{client: s3vectors.NewFromConfig(cfg), bucket: bucket, index: index}, nil
}

// Put writes or overwrites the vector for a file id, tagged with its owner for query filtering.
func (s *S3Vectors) Put(ctx context.Context, id string, owner string, vector []float32) error {
	_, err := s.client.PutVectors(ctx, &s3vectors.PutVectorsInput{
		VectorBucketName: aws.String(s.bucket),
		IndexName:        aws.String(s.index),
		Vectors: []types.PutInputVector{{
			Key:      aws.String(id),
			Data:     &types.VectorDataMemberFloat32{Value: vector},
			Metadata: document.NewLazyDocument(map[string]any{ownerKey: owner}),
		}},
	})
	if err != nil {
		return fmt.Errorf("put vector %q: %w", id, err)
	}
	return nil
}

// Query returns the ids of the owner's nearest vectors to the query, closest first.
func (s *S3Vectors) Query(ctx context.Context, owner string, vector []float32, topK int32) ([]string, error) {
	out, err := s.client.QueryVectors(ctx, &s3vectors.QueryVectorsInput{
		VectorBucketName: aws.String(s.bucket),
		IndexName:        aws.String(s.index),
		QueryVector:      &types.VectorDataMemberFloat32{Value: vector},
		TopK:             aws.Int32(topK),
		Filter:           document.NewLazyDocument(map[string]any{ownerKey: owner}),
	})
	if err != nil {
		return nil, fmt.Errorf("query vectors: %w", err)
	}

	ids := make([]string, 0, len(out.Vectors))
	for _, match := range out.Vectors {
		if match.Key != nil {
			ids = append(ids, *match.Key)
		}
	}
	return ids, nil
}

// Delete removes the vector for a file id.
func (s *S3Vectors) Delete(ctx context.Context, id string) error {
	_, err := s.client.DeleteVectors(ctx, &s3vectors.DeleteVectorsInput{
		VectorBucketName: aws.String(s.bucket),
		IndexName:        aws.String(s.index),
		Keys:             []string{id},
	})
	if err != nil {
		return fmt.Errorf("delete vector %q: %w", id, err)
	}
	return nil
}
