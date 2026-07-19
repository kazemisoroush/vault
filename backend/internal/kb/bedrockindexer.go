package kb

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
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

// Sync starts an ingestion job for the data source and waits until it reaches a terminal state. On
// a completed job it returns the files the job could not index, so the caller may advance the rest
// and mark those failed. If a job is already running, it starts nothing and returns not-completed,
// leaving the in-flight job and a later run to finish; the data source allows one job at a time.
func (i *BedrockIndexer) Sync(ctx context.Context) (SyncResult, error) {
	out, err := i.client.StartIngestionJob(ctx, &bedrockagent.StartIngestionJobInput{
		KnowledgeBaseId: aws.String(i.kbID),
		DataSourceId:    aws.String(i.dataSourceID),
	})
	if err != nil {
		var conflict *types.ConflictException
		if errors.As(err, &conflict) {
			return SyncResult{}, nil
		}
		return SyncResult{}, fmt.Errorf("start ingestion job: %w", err)
	}

	result, err := i.wait(ctx, aws.ToString(out.IngestionJob.IngestionJobId))
	if err != nil {
		return SyncResult{}, fmt.Errorf("await ingestion job: %w", err)
	}
	return result, nil
}

// wait polls the ingestion job until it reaches a terminal state or the context is done. On a
// completed job it parses the failure reasons for the file ids the job could not index.
func (i *BedrockIndexer) wait(ctx context.Context, jobID string) (SyncResult, error) {
	for {
		out, err := i.client.GetIngestionJob(ctx, &bedrockagent.GetIngestionJobInput{
			KnowledgeBaseId: aws.String(i.kbID),
			DataSourceId:    aws.String(i.dataSourceID),
			IngestionJobId:  aws.String(jobID),
		})
		if err != nil {
			return SyncResult{}, fmt.Errorf("get ingestion job %q: %w", jobID, err)
		}
		switch out.IngestionJob.Status {
		case types.IngestionJobStatusComplete:
			return SyncResult{Completed: true, FailedFileIDs: failedFileIDs(out.IngestionJob.FailureReasons)}, nil
		case types.IngestionJobStatusFailed, types.IngestionJobStatusStopped, types.IngestionJobStatusStopping:
			return SyncResult{}, fmt.Errorf("ingestion job %q ended in status %s", jobID, out.IngestionJob.Status)
		}
		select {
		case <-ctx.Done():
			return SyncResult{}, fmt.Errorf("wait for ingestion job %q: %w", jobID, ctx.Err())
		case <-time.After(i.pollInterval):
		}
	}
}

// s3URIPattern matches the S3 URIs the ingestion failure reasons list for the objects that failed.
var s3URIPattern = regexp.MustCompile(`s3://[^\s,\]"]+`)

// failedFileIDs pulls the file ids out of a job's failure reasons. Each reason lists the failed
// objects as S3 URIs, and a file id is the object's key basename with any metadata suffix removed,
// so the caller can mark exactly those files failed rather than searchable.
func failedFileIDs(reasons []string) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, reason := range reasons {
		for _, uri := range s3URIPattern.FindAllString(reason, -1) {
			id := strings.TrimSuffix(path.Base(uri), ".metadata.json")
			if id != "" && id != "." && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}
