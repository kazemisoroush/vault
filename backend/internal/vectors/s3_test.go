package vectors

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors"
	"github.com/aws/aws-sdk-go-v2/service/s3vectors/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClient captures calls and returns canned query results.
type fakeClient struct {
	putKey     string
	putOwner   string
	queryOwner string
	deleteKeys []string
	queryKeys  []string
}

func (f *fakeClient) PutVectors(_ context.Context, in *s3vectors.PutVectorsInput, _ ...func(*s3vectors.Options)) (*s3vectors.PutVectorsOutput, error) {
	f.putKey = *in.Vectors[0].Key
	meta := map[string]string{}
	_ = in.Vectors[0].Metadata.UnmarshalSmithyDocument(&meta)
	f.putOwner = meta["owner"]
	return &s3vectors.PutVectorsOutput{}, nil
}

func (f *fakeClient) QueryVectors(_ context.Context, in *s3vectors.QueryVectorsInput, _ ...func(*s3vectors.Options)) (*s3vectors.QueryVectorsOutput, error) {
	filter := map[string]string{}
	_ = in.Filter.UnmarshalSmithyDocument(&filter)
	f.queryOwner = filter["owner"]
	vectors := make([]types.QueryOutputVector, 0, len(f.queryKeys))
	for _, key := range f.queryKeys {
		vectors = append(vectors, types.QueryOutputVector{Key: aws.String(key)})
	}
	return &s3vectors.QueryVectorsOutput{Vectors: vectors}, nil
}

func (f *fakeClient) DeleteVectors(_ context.Context, in *s3vectors.DeleteVectorsInput, _ ...func(*s3vectors.Options)) (*s3vectors.DeleteVectorsOutput, error) {
	f.deleteKeys = in.Keys
	return &s3vectors.DeleteVectorsOutput{}, nil
}

func TestPutWritesUnderTheFileID(t *testing.T) {
	// Arrange
	fake := &fakeClient{}
	store := &S3Vectors{client: fake, bucket: "b", index: "i"}

	// Act
	err := store.Put(context.Background(), "f_123", "alice", []float32{0.1, 0.2})

	// Assert: the vector is tagged with its owner so queries can filter on it.
	require.NoError(t, err)
	assert.Equal(t, "f_123", fake.putKey)
	assert.Equal(t, "alice", fake.putOwner)
}

func TestQueryFiltersToTheOwner(t *testing.T) {
	// Arrange
	fake := &fakeClient{queryKeys: []string{"f_a", "f_b"}}
	store := &S3Vectors{client: fake, bucket: "b", index: "i"}

	// Act
	ids, err := store.Query(context.Background(), "alice", []float32{0.1}, 10)

	// Assert: the query carries the owner filter, so only the owner's vectors are searched.
	require.NoError(t, err)
	assert.Equal(t, []string{"f_a", "f_b"}, ids)
	assert.Equal(t, "alice", fake.queryOwner)
}

func TestDeleteRemovesTheFileID(t *testing.T) {
	// Arrange
	fake := &fakeClient{}
	store := &S3Vectors{client: fake, bucket: "b", index: "i"}

	// Act
	err := store.Delete(context.Background(), "f_123")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"f_123"}, fake.deleteKeys)
}
