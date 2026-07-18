package kb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagent"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagent/types"
)

// defaultPollInterval is how often the indexer checks a running ingestion job for completion.
const defaultPollInterval = 10 * time.Second

// indexerClient is the slice of the Bedrock agent API the BedrockIndexer uses, kept small to fake
// in tests.
type indexerClient interface {
	StartIngestionJob(ctx context.Context, in *bedrockagent.StartIngestionJobInput, optFns ...func(*bedrockagent.Options)) (*bedrockagent.StartIngestionJobOutput, error)
	GetIngestionJob(ctx context.Context, in *bedrockagent.GetIngestionJobInput, optFns ...func(*bedrockagent.Options)) (*bedrockagent.GetIngestionJobOutput, error)
}

// BedrockIndexer starts a Knowledge Base ingestion job for the data source and waits until it
// finishes, so the files that landed in S3 become searchable.
type BedrockIndexer struct {
	client       indexerClient
	kbID         string
	dataSourceID string
	pollInterval time.Duration
}

// NewBedrockIndexer builds a BedrockIndexer for one Knowledge Base data source.
func NewBedrockIndexer(client indexerClient, kbID string, dataSourceID string) *BedrockIndexer {
	return &BedrockIndexer{client: client, kbID: kbID, dataSourceID: dataSourceID, pollInterval: defaultPollInterval}
}

// Sync starts an ingestion job for the data source and waits until it reaches a terminal state. It
// returns true when a job it started completed, so the caller may advance the files it snapshotted
// before the call. If a job is already running, it starts nothing and returns false, leaving the
// in-flight job and a later run to finish the work; the data source allows only one job at a time.
func (i *BedrockIndexer) Sync(ctx context.Context) (bool, error) {
	out, err := i.client.StartIngestionJob(ctx, &bedrockagent.StartIngestionJobInput{
		KnowledgeBaseId: aws.String(i.kbID),
		DataSourceId:    aws.String(i.dataSourceID),
	})
	if err != nil {
		var conflict *types.ConflictException
		if errors.As(err, &conflict) {
			return false, nil
		}
		return false, fmt.Errorf("start ingestion job: %w", err)
	}

	completed, err := i.wait(ctx, aws.ToString(out.IngestionJob.IngestionJobId))
	if err != nil {
		return false, fmt.Errorf("await ingestion job: %w", err)
	}
	return completed, nil
}

// wait polls the ingestion job until it reaches a terminal state or the context is done.
func (i *BedrockIndexer) wait(ctx context.Context, jobID string) (bool, error) {
	for {
		out, err := i.client.GetIngestionJob(ctx, &bedrockagent.GetIngestionJobInput{
			KnowledgeBaseId: aws.String(i.kbID),
			DataSourceId:    aws.String(i.dataSourceID),
			IngestionJobId:  aws.String(jobID),
		})
		if err != nil {
			return false, fmt.Errorf("get ingestion job %q: %w", jobID, err)
		}
		switch out.IngestionJob.Status {
		case types.IngestionJobStatusComplete:
			return true, nil
		case types.IngestionJobStatusFailed, types.IngestionJobStatusStopped, types.IngestionJobStatusStopping:
			return false, fmt.Errorf("ingestion job %q ended in status %s", jobID, out.IngestionJob.Status)
		}
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("wait for ingestion job %q: %w", jobID, ctx.Err())
		case <-time.After(i.pollInterval):
		}
	}
}
