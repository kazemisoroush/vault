package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// LambdaEnqueuer runs a check by asynchronously invoking this same Lambda with a Task payload,
// the same single-function pattern the S3 ingest trigger uses. The API request returns
// immediately; the async invocation runs the pipeline with the full function timeout.
type LambdaEnqueuer struct {
	client   *lambda.Client
	function string
}

// NewLambdaEnqueuer builds an enqueuer that self-invokes the named function.
func NewLambdaEnqueuer(client *lambda.Client, function string) *LambdaEnqueuer {
	return &LambdaEnqueuer{client: client, function: function}
}

// Enqueue fires the check task as an async self-invocation.
func (e *LambdaEnqueuer) Enqueue(ctx context.Context, checkID string, ownerID string) error {
	payload, err := json.Marshal(NewTask(checkID, ownerID))
	if err != nil {
		return fmt.Errorf("marshal check task: %w", err)
	}
	_, err = e.client.Invoke(ctx, &lambda.InvokeInput{
		FunctionName:   aws.String(e.function),
		InvocationType: types.InvocationTypeEvent,
		Payload:        payload,
	})
	if err != nil {
		return fmt.Errorf("enqueue check %q: %w", checkID, err)
	}
	return nil
}

// localRunTimeout bounds one check pipeline when it runs in-process.
const localRunTimeout = 5 * time.Minute

// LocalEnqueuer runs the check in a goroutine, for the local development server where there is
// no Lambda to self-invoke.
type LocalEnqueuer struct {
	runner *Runner
}

// NewLocalEnqueuer builds an enqueuer over an in-process runner.
func NewLocalEnqueuer(runner *Runner) *LocalEnqueuer {
	return &LocalEnqueuer{runner: runner}
}

// Enqueue starts the pipeline in the background. The goroutine gets its own context: the HTTP
// request that created the check finishes long before the pipeline does.
func (e *LocalEnqueuer) Enqueue(_ context.Context, checkID string, ownerID string) error {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), localRunTimeout)
		defer cancel()
		if err := e.runner.Run(ctx, checkID, ownerID); err != nil {
			log.Printf("run check %s: %v", checkID, err)
		}
	}()
	return nil
}
