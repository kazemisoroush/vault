// Package router sends each of the Lambda's triggers to the right handler.
package router

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

// KBSyncer advances landed files into the Knowledge Base on a schedule.
type KBSyncer interface {
	Sync(ctx context.Context) error
}

// EventRouter routes an S3 event to ingestion, a scheduled event to the Knowledge Base syncer, a
// check task to the check verifier, and every other event to the HTTP proxy.
type EventRouter struct {
	proxy    Proxy
	ingester Ingester
	syncer   KBSyncer
	checks   CheckVerifier
}

// NewEventRouter builds an EventRouter over the HTTP proxy, the ingester, the Knowledge Base syncer,
// and the check verifier.
func NewEventRouter(proxy Proxy, ingester Ingester, syncer KBSyncer, checkVerifier CheckVerifier) *EventRouter {
	return &EventRouter{proxy: proxy, ingester: ingester, syncer: syncer, checks: checkVerifier}
}

// Route sends one raw Lambda event to the handler for its source.
func (r *EventRouter) Route(ctx context.Context, raw json.RawMessage) (any, error) {
	// Uploads arrive through the ingest queue, so an SQS message wraps one S3 notification. A record
	// that fails to ingest returns an error, so the queue redelivers it and, after enough tries,
	// dead-letters it rather than losing the file.
	if isSQSEvent(raw) {
		var event events.SQSEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("unmarshal SQS event: %w", err)
		}
		for _, record := range event.Records {
			var s3Event events.S3Event
			if err := json.Unmarshal([]byte(record.Body), &s3Event); err != nil {
				return nil, fmt.Errorf("unmarshal S3 event from queue message: %w", err)
			}
			if err := r.ingester.Handle(ctx, s3Event); err != nil {
				return nil, fmt.Errorf("ingest queued S3 event: %w", err)
			}
		}
		return nil, nil
	}

	if isS3Event(raw) {
		var event events.S3Event
		if err := json.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("unmarshal S3 event: %w", err)
		}
		if err := r.ingester.Handle(ctx, event); err != nil {
			return nil, fmt.Errorf("ingest S3 event: %w", err)
		}
		return nil, nil
	}

	if isScheduledEvent(raw) {
		if err := r.syncer.Sync(ctx); err != nil {
			return nil, fmt.Errorf("knowledge base sync: %w", err)
		}
		return nil, nil
	}

	if task, ok := checkTask(raw); ok {
		if err := r.checks.Verify(ctx, task.CheckID, task.OwnerID); err != nil {
			return nil, fmt.Errorf("verify check task: %w", err)
		}
		return nil, nil
	}

	var request events.APIGatewayV2HTTPRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, fmt.Errorf("unmarshal API request: %w", err)
	}
	resp, err := r.proxy(ctx, request)
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

// sqsEventProbe sniffs the event source of the first record.
type sqsEventProbe struct {
	Records []struct {
		EventSource string `json:"eventSource"`
	} `json:"Records"`
}

// isSQSEvent reports whether the raw event is an SQS event, which is how upload notifications reach
// the Lambda once they pass through the ingest queue.
func isSQSEvent(raw json.RawMessage) bool {
	var probe sqsEventProbe
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return len(probe.Records) > 0 && probe.Records[0].EventSource == "aws:sqs"
}

// isS3Event reports whether the raw event is an S3 event.
func isS3Event(raw json.RawMessage) bool {
	var probe s3EventProbe
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return len(probe.Records) > 0 && probe.Records[0].EventSource == "aws:s3"
}

// scheduledEventProbe sniffs the source of an EventBridge event.
type scheduledEventProbe struct {
	Source string `json:"source"`
}

// isScheduledEvent reports whether the raw event is the EventBridge schedule that drives the
// Knowledge Base sync. EventBridge stamps scheduled rules with the aws.events source.
func isScheduledEvent(raw json.RawMessage) bool {
	var probe scheduledEventProbe
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return probe.Source == "aws.events"
}

// checkTask sniffs a raw event for a check task payload.
func checkTask(raw json.RawMessage) (checks.Task, bool) {
	var task checks.Task
	if err := json.Unmarshal(raw, &task); err != nil {
		return checks.Task{}, false
	}
	return task, task.Task == checks.TaskName && task.CheckID != ""
}
