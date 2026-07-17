// Package transport adapts the Lambda's triggers to the right handler.
package transport

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"

	"github.com/kazemisoroush/vault/backend/internal/checks"
)

// Proxy handles an API Gateway HTTP request.
type Proxy func(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)

// CheckVerifier verifies one queued check pipeline.
type CheckVerifier interface {
	Verify(ctx context.Context, checkID string, ownerID string) error
}

// Transport routes an S3 event to ingestion, a check task to the check verifier, and every other
// event to the HTTP proxy.
type Transport struct {
	proxy    Proxy
	ingester Ingester
	checks   CheckVerifier
}

// NewTransport builds a Transport over the HTTP proxy, the ingester, and the check verifier.
func NewTransport(proxy Proxy, ingester Ingester, checkVerifier CheckVerifier) *Transport {
	return &Transport{proxy: proxy, ingester: ingester, checks: checkVerifier}
}

// Handle routes one raw Lambda event by its source.
func (t *Transport) Handle(ctx context.Context, raw json.RawMessage) (any, error) {
	if isS3Event(raw) {
		var event events.S3Event
		if err := json.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("unmarshal S3 event: %w", err)
		}
		if err := t.ingester.Handle(ctx, event); err != nil {
			return nil, fmt.Errorf("ingest S3 event: %w", err)
		}
		return nil, nil
	}

	if task, ok := checkTask(raw); ok {
		if err := t.checks.Verify(ctx, task.CheckID, task.OwnerID); err != nil {
			return nil, fmt.Errorf("verify check task: %w", err)
		}
		return nil, nil
	}

	var request events.APIGatewayV2HTTPRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, fmt.Errorf("unmarshal API request: %w", err)
	}
	resp, err := t.proxy(ctx, request)
	if err != nil {
		return resp, fmt.Errorf("proxy request: %w", err)
	}
	return resp, nil
}

// s3EventProbe sniffs just enough of a raw event to detect an S3 notification.
type s3EventProbe struct {
	Records []s3EventProbeRecord `json:"Records"`
}

// s3EventProbeRecord carries one record's event source.
type s3EventProbeRecord struct {
	EventSource string `json:"eventSource"`
}

// isS3Event reports whether the raw event is an S3 event.
func isS3Event(raw json.RawMessage) bool {
	var probe s3EventProbe
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return len(probe.Records) > 0 && probe.Records[0].EventSource == "aws:s3"
}

// checkTask sniffs a raw event for a check task payload.
func checkTask(raw json.RawMessage) (checks.Task, bool) {
	var task checks.Task
	if err := json.Unmarshal(raw, &task); err != nil {
		return checks.Task{}, false
	}
	return task, task.Task == checks.TaskName && task.CheckID != ""
}
