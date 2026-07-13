package vectors

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors/document"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors/types"
)

const (
	// ownerKey scopes a query to its owner; fileKey lets a file's chunks be found for deletion.
	ownerKey = "ownerId"
	fileKey  = "fileId"
	// chunkSep joins a file id and its chunk index into a vector key, "<fileId>#<n>".
	chunkSep = "#"
	// overFetch is how many chunk hits to pull per requested file, so deduping chunk hits back to
	// files still yields topK distinct files even when several chunks of one file rank together.
	overFetch = int32(20)
	// maxFetch caps the chunk hits pulled for one query, a ceiling on the over-fetch above.
	maxFetch = int32(500)
	// chunkListLimit bounds the chunks enumerated for a file when deleting it; it sits above the
	// chunk package's per-file cap so a delete always sees every one of a file's chunks.
	chunkListLimit = int32(512)
	// indexDimensions is the embedding width the vector index was created with, which is Amazon
	// Titan Text v2's 1024. It is used only to shape the probe vector that lists a file's chunks
	// for deletion, where direction is irrelevant because the metadata filter does the selecting.
	indexDimensions = 1024
)

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

// Put writes a file's chunk vectors, each keyed "<fileId>#<n>" and tagged with the owner (for query
// filtering) and the file id (so the file's chunks can be found for deletion). Re-putting the same
// content-addressed file overwrites the same keys.
func (s *S3Vectors) Put(ctx context.Context, fileID string, ownerID string, vectors [][]float32) error {
	if len(vectors) == 0 {
		return nil
	}
	inputs := make([]types.PutInputVector, 0, len(vectors))
	for i, vector := range vectors {
		inputs = append(inputs, types.PutInputVector{
			Key:      aws.String(chunkKey(fileID, i)),
			Data:     &types.VectorDataMemberFloat32{Value: vector},
			Metadata: document.NewLazyDocument(map[string]any{ownerKey: ownerID, fileKey: fileID}),
		})
	}
	_, err := s.client.PutVectors(ctx, &s3vectors.PutVectorsInput{
		VectorBucketName: aws.String(s.bucket),
		IndexName:        aws.String(s.index),
		Vectors:          inputs,
	})
	if err != nil {
		return fmt.Errorf("put %d vectors for %q: %w", len(vectors), fileID, err)
	}
	return nil
}

// Query returns the ids of the owner's nearest files to the query, closest first. It over-fetches
// chunk hits and dedupes them back to distinct files, so a file ranks by its single closest chunk
// and appears at most once, and at most topK files are returned.
func (s *S3Vectors) Query(ctx context.Context, ownerID string, vector []float32, topK int32) ([]string, error) {
	fetch := topK * overFetch
	if fetch > maxFetch {
		fetch = maxFetch
	}
	if fetch < topK {
		fetch = topK
	}
	out, err := s.client.QueryVectors(ctx, &s3vectors.QueryVectorsInput{
		VectorBucketName: aws.String(s.bucket),
		IndexName:        aws.String(s.index),
		QueryVector:      &types.VectorDataMemberFloat32{Value: vector},
		TopK:             aws.Int32(fetch),
		Filter:           document.NewLazyDocument(map[string]any{ownerKey: ownerID}),
	})
	if err != nil {
		return nil, fmt.Errorf("query vectors: %w", err)
	}

	seen := make(map[string]struct{}, topK)
	ids := make([]string, 0, topK)
	for _, match := range out.Vectors {
		if match.Key == nil {
			continue
		}
		fileID := fileIDFromKey(*match.Key)
		if _, ok := seen[fileID]; ok {
			continue
		}
		seen[fileID] = struct{}{}
		ids = append(ids, fileID)
		if int32(len(ids)) >= topK {
			break
		}
	}
	return ids, nil
}

// Delete removes every chunk vector of a file. It finds the file's chunk keys through a metadata
// filter on the file id (list has no filter, so a filtered query does the selecting; the probe
// vector's direction is irrelevant), then deletes them. A file with no vectors is a no-op.
func (s *S3Vectors) Delete(ctx context.Context, fileID string) error {
	out, err := s.client.QueryVectors(ctx, &s3vectors.QueryVectorsInput{
		VectorBucketName: aws.String(s.bucket),
		IndexName:        aws.String(s.index),
		QueryVector:      &types.VectorDataMemberFloat32{Value: probeVector()},
		TopK:             aws.Int32(chunkListLimit),
		Filter:           document.NewLazyDocument(map[string]any{fileKey: fileID}),
	})
	if err != nil {
		return fmt.Errorf("list chunks of %q: %w", fileID, err)
	}

	keys := make([]string, 0, len(out.Vectors))
	for _, match := range out.Vectors {
		if match.Key != nil {
			keys = append(keys, *match.Key)
		}
	}
	if len(keys) == 0 {
		return nil
	}

	_, err = s.client.DeleteVectors(ctx, &s3vectors.DeleteVectorsInput{
		VectorBucketName: aws.String(s.bucket),
		IndexName:        aws.String(s.index),
		Keys:             keys,
	})
	if err != nil {
		return fmt.Errorf("delete chunks of %q: %w", fileID, err)
	}
	return nil
}

// chunkKey is the vector key for a file's nth chunk.
func chunkKey(fileID string, index int) string {
	return fileID + chunkSep + strconv.Itoa(index)
}

// fileIDFromKey recovers the file id from a chunk key, so chunk hits dedupe back to their file.
func fileIDFromKey(key string) string {
	id, _, _ := strings.Cut(key, chunkSep)
	return id
}

// probeVector is a unit vector of the index width, used only to enumerate a file's chunks for
// deletion. Its direction does not matter: the metadata filter, not similarity, selects the chunks.
func probeVector() []float32 {
	probe := make([]float32, indexDimensions)
	for i := range probe {
		probe[i] = 1
	}
	return probe
}
