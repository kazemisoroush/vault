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
	putKeys     []string
	putOwner    string
	putFile     string
	queryFilter map[string]string
	queryTopK   int32
	deleteKeys  []string
	queryKeys   []string
}

func (f *fakeClient) PutVectors(_ context.Context, in *s3vectors.PutVectorsInput, _ ...func(*s3vectors.Options)) (*s3vectors.PutVectorsOutput, error) {
	for _, v := range in.Vectors {
		f.putKeys = append(f.putKeys, *v.Key)
	}
	meta := map[string]string{}
	_ = in.Vectors[0].Metadata.UnmarshalSmithyDocument(&meta)
	f.putOwner = meta["ownerId"]
	f.putFile = meta["fileId"]
	return &s3vectors.PutVectorsOutput{}, nil
}

func (f *fakeClient) QueryVectors(_ context.Context, in *s3vectors.QueryVectorsInput, _ ...func(*s3vectors.Options)) (*s3vectors.QueryVectorsOutput, error) {
	f.queryFilter = map[string]string{}
	_ = in.Filter.UnmarshalSmithyDocument(&f.queryFilter)
	if in.TopK != nil {
		f.queryTopK = *in.TopK
	}
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

func TestPutKeysEachChunkUnderTheFileIDWithOwnerAndFileTags(t *testing.T) {
	// Arrange
	fake := &fakeClient{}
	store := &S3Vectors{client: fake, bucket: "b", index: "i"}

	// Act: a file with two chunks.
	err := store.Put(context.Background(), "f_123", "alice", [][]float32{{0.1, 0.2}, {0.3, 0.4}})

	// Assert: one key per chunk, and each carries the owner and file tags.
	require.NoError(t, err)
	assert.Equal(t, []string{"f_123#0", "f_123#1"}, fake.putKeys)
	assert.Equal(t, "alice", fake.putOwner)
	assert.Equal(t, "f_123", fake.putFile)
}

func TestPutWithNoVectorsIsANoOp(t *testing.T) {
	// Arrange
	fake := &fakeClient{}
	store := &S3Vectors{client: fake, bucket: "b", index: "i"}

	// Act
	err := store.Put(context.Background(), "f_123", "alice", nil)

	// Assert: nothing is written.
	require.NoError(t, err)
	assert.Empty(t, fake.putKeys)
}

func TestQueryDedupesChunkHitsBackToFilesAndFiltersToOwner(t *testing.T) {
	// Arrange: three chunk hits, two of them from the same file.
	fake := &fakeClient{queryKeys: []string{"f_a#3", "f_a#0", "f_b#1"}}
	store := &S3Vectors{client: fake, bucket: "b", index: "i"}

	// Act
	ids, err := store.Query(context.Background(), "alice", []float32{0.1}, 10)

	// Assert: distinct file ids in hit order, and the query carries the owner filter.
	require.NoError(t, err)
	assert.Equal(t, []string{"f_a", "f_b"}, ids)
	assert.Equal(t, "alice", fake.queryFilter["ownerId"])
}

func TestQueryReturnsAtMostTopKDistinctFiles(t *testing.T) {
	// Arrange: hits from three files, but only two are wanted.
	fake := &fakeClient{queryKeys: []string{"f_a#0", "f_b#0", "f_c#0"}}
	store := &S3Vectors{client: fake, bucket: "b", index: "i"}

	// Act
	ids, err := store.Query(context.Background(), "alice", []float32{0.1}, 2)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"f_a", "f_b"}, ids)
}

func TestDeleteRemovesEveryChunkOfTheFile(t *testing.T) {
	// Arrange: the file's chunks come back from the filtered lookup.
	fake := &fakeClient{queryKeys: []string{"f_123#0", "f_123#1", "f_123#2"}}
	store := &S3Vectors{client: fake, bucket: "b", index: "i"}

	// Act
	err := store.Delete(context.Background(), "f_123")

	// Assert: the lookup filters by file id, and every returned chunk key is deleted.
	require.NoError(t, err)
	assert.Equal(t, "f_123", fake.queryFilter["fileId"])
	assert.Equal(t, []string{"f_123#0", "f_123#1", "f_123#2"}, fake.deleteKeys)
}

func TestDeleteWithNoChunksIsANoOp(t *testing.T) {
	// Arrange: the file has no vectors.
	fake := &fakeClient{}
	store := &S3Vectors{client: fake, bucket: "b", index: "i"}

	// Act
	err := store.Delete(context.Background(), "f_123")

	// Assert: nothing is deleted.
	require.NoError(t, err)
	assert.Nil(t, fake.deleteKeys)
}
