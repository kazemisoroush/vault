// Lambda entrypoint behind an API Gateway HTTP API (payload format 2.0).
package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/kazemisoroush/vault/backend/internal/blob"
	appconfig "github.com/kazemisoroush/vault/backend/internal/config"
	"github.com/kazemisoroush/vault/backend/internal/extract"
	"github.com/kazemisoroush/vault/backend/internal/handler"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/ingest"
	"github.com/kazemisoroush/vault/backend/internal/router"
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

	routes, err := handler.Guard(ctx, cfg, h.Routes())
	if err != nil {
		log.Fatalf("configure auth: %v", err)
	}
	proxy := httpadapter.NewV2(routes).ProxyWithContext

	extractor, err := extract.NewClaudeExtractor(ctx, cfg.BedrockRegion, cfg.ExtractorModel)
	if err != nil {
		log.Fatalf("configure extractor: %v", err)
	}
	ingester := ingest.New(idx, blobs, extractor)

	dispatcher := router.New(proxy, ingester)
	lambda.Start(dispatcher.Handle)
}
