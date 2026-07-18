package kb

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagent"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagent/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIndexerClient stands in for the Bedrock agent client: it starts a job (or fails to) and
// reports a fixed job status.
type fakeIndexerClient struct {
	startErr error
	jobID    string
	status   types.IngestionJobStatus
}

func (f *fakeIndexerClient) StartIngestionJob(_ context.Context, _ *bedrockagent.StartIngestionJobInput, _ ...func(*bedrockagent.Options)) (*bedrockagent.StartIngestionJobOutput, error) {
	if f.startErr != nil {
		return nil, f.startErr
	}
	return &bedrockagent.StartIngestionJobOutput{IngestionJob: &types.IngestionJob{IngestionJobId: aws.String(f.jobID)}}, nil
}

func (f *fakeIndexerClient) GetIngestionJob(_ context.Context, _ *bedrockagent.GetIngestionJobInput, _ ...func(*bedrockagent.Options)) (*bedrockagent.GetIngestionJobOutput, error) {
	return &bedrockagent.GetIngestionJobOutput{IngestionJob: &types.IngestionJob{Status: f.status}}, nil
}

func TestBedrockIndexerReportsACompletedJob(t *testing.T) {
	// Arrange: the job starts and reaches COMPLETE.
	client := &fakeIndexerClient{jobID: "job-1", status: types.IngestionJobStatusComplete}
	indexer := NewBedrockIndexer(client, "kb-1", "ds-1")

	// Act
	completed, err := indexer.Sync(context.Background())

	// Assert
	require.NoError(t, err)
	assert.True(t, completed)
}

func TestBedrockIndexerSkipsWhenAJobIsAlreadyRunning(t *testing.T) {
	// Arrange: the data source already has a job, so StartIngestionJob conflicts.
	client := &fakeIndexerClient{startErr: &types.ConflictException{}}
	indexer := NewBedrockIndexer(client, "kb-1", "ds-1")

	// Act
	completed, err := indexer.Sync(context.Background())

	// Assert: no error, but no job completed this call.
	require.NoError(t, err)
	assert.False(t, completed)
}

func TestBedrockIndexerErrorsOnAFailedJob(t *testing.T) {
	// Arrange: the job starts but ends FAILED.
	client := &fakeIndexerClient{jobID: "job-1", status: types.IngestionJobStatusFailed}
	indexer := NewBedrockIndexer(client, "kb-1", "ds-1")

	// Act
	completed, err := indexer.Sync(context.Background())

	// Assert
	assert.Error(t, err)
	assert.False(t, completed)
}
