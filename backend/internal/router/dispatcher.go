// Package router dispatches a raw Lambda event to the right handler.
package router

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

// Proxy handles an API Gateway HTTP request.
type Proxy func(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)

// Dispatcher routes S3 events to ingestion and everything else to the HTTP proxy.
type Dispatcher struct {
	proxy    Proxy
	ingester Ingester
}

// New builds a Dispatcher over the HTTP proxy and the ingester.
func New(proxy Proxy, ingester Ingester) *Dispatcher {
	return &Dispatcher{proxy: proxy, ingester: ingester}
}

// Handle routes one raw Lambda event by its type.
func (d *Dispatcher) Handle(ctx context.Context, raw json.RawMessage) (any, error) {
	if isS3Event(raw) {
		var event events.S3Event
		if err := json.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("unmarshal S3 event: %w", err)
		}
		return nil, d.ingester.Handle(ctx, event)
	}

	var request events.APIGatewayV2HTTPRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, fmt.Errorf("unmarshal API request: %w", err)
	}
	resp, err := d.proxy(ctx, request)
	if err != nil {
		return resp, fmt.Errorf("proxy request: %w", err)
	}
	return resp, nil
}

// isS3Event reports whether the raw event is an S3 event.
func isS3Event(raw json.RawMessage) bool {
	var probe struct {
		Records []struct {
			EventSource string `json:"eventSource"`
		} `json:"Records"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return len(probe.Records) > 0 && probe.Records[0].EventSource == "aws:s3"
}
