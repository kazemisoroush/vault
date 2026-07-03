// Lambda entrypoint behind an API Gateway HTTP API (payload format 2.0).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	appconfig "github.com/kazemisoroush/vault/backend/internal/config"
	"github.com/kazemisoroush/vault/backend/internal/extract"
	"github.com/kazemisoroush/vault/backend/internal/handler"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/ingest"
)

func main() {
	ctx := context.Background()
	cfg := appconfig.Load()

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("load AWS config: %v", err)
	}

	idx := index.NewDynamoIndex(dynamodb.NewFromConfig(awsCfg), cfg.Table)
	blobs := blob.NewS3Store(s3.NewFromConfig(awsCfg), cfg.Bucket)
	h := handler.New(idx, blobs)

	routes, err := guard(ctx, cfg, h.Routes())
	if err != nil {
		log.Fatalf("configure auth: %v", err)
	}
	proxy := httpadapter.NewV2(routes).ProxyWithContext

	extractor, err := extract.NewClaudeExtractor(ctx, cfg.BedrockRegion, cfg.ExtractorModel)
	if err != nil {
		log.Fatalf("configure extractor: %v", err)
	}
	ingester := ingest.New(idx, blobs, extractor)

	lambda.Start(func(ctx context.Context, raw json.RawMessage) (any, error) {
		if isS3Event(raw) {
			var event events.S3Event
			if err := json.Unmarshal(raw, &event); err != nil {
				return nil, fmt.Errorf("unmarshal S3 event: %w", err)
			}
			return nil, ingester.Handle(ctx, event)
		}

		var request events.APIGatewayV2HTTPRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, fmt.Errorf("unmarshal API request: %w", err)
		}
		resp, err := proxy(ctx, request)
		if err != nil {
			return resp, fmt.Errorf("proxy request: %w", err)
		}
		return resp, nil
	})
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

// guard wraps the routes with JWT auth, failing closed unless auth is opted out.
func guard(ctx context.Context, cfg appconfig.Config, routes http.Handler) (http.Handler, error) {
	if cfg.AuthDisabled {
		log.Print("auth explicitly disabled via VAULT_AUTH_DISABLED; serving without authentication")
		return routes, nil
	}
	if !cfg.AuthEnabled() {
		return nil, errors.New("auth not configured: set VAULT_JWT_ISSUER and VAULT_JWT_CLIENT_ID, or set VAULT_AUTH_DISABLED=true to run without auth")
	}

	keyFunc, err := auth.NewCognitoKeyFunc(ctx, cfg.JWTIssuer)
	if err != nil {
		return nil, err
	}
	verifier := auth.NewVerifier(cfg.JWTIssuer, cfg.JWTClientID, keyFunc)
	return handler.RequireAuth(routes, verifier), nil
}
