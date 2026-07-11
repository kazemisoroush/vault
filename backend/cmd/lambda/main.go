// Lambda entrypoint behind an API Gateway HTTP API (payload format 2.0).
package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/kazemisoroush/vault/backend/internal/agent"
	"github.com/kazemisoroush/vault/backend/internal/api"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/calls"
	appconfig "github.com/kazemisoroush/vault/backend/internal/config"
	"github.com/kazemisoroush/vault/backend/internal/embed"
	"github.com/kazemisoroush/vault/backend/internal/extract"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/ingest"
	"github.com/kazemisoroush/vault/backend/internal/llm"
	"github.com/kazemisoroush/vault/backend/internal/telemetry"
	"github.com/kazemisoroush/vault/backend/internal/transport"
	"github.com/kazemisoroush/vault/backend/internal/vectors"
)

func main() {
	ctx := context.Background()
	cfg := appconfig.Load()

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("load AWS config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	idx := index.NewDynamoIndex(dynamoClient, cfg.Table)
	blobs := blob.NewS3Store(s3.NewFromConfig(awsCfg), cfg.Bucket)
	recorder := calls.NewDynamoCalls(dynamoClient, cfg.CallsTable)

	embedder, err := embed.NewTitanEmbedder(ctx, cfg.BedrockRegion, cfg.EmbedModel, recorder)
	if err != nil {
		log.Fatalf("configure embedder: %v", err)
	}
	vectorStore, err := vectors.NewS3Vectors(ctx, cfg.BedrockRegion, cfg.VectorBucket, cfg.VectorIndex)
	if err != nil {
		log.Fatalf("configure vector store: %v", err)
	}

	answerer := agent.NewAgent(llm.NewModel(cfg.BedrockRegion, cfg.RerankModel, agent.ModelOp, recorder), embedder, vectorStore, idx)

	apiHandler, err := api.NewHandler(ctx, cfg, idx, blobs, vectorStore, answerer, recorder, telemetry.NewEMFEmitter(os.Stdout))
	if err != nil {
		log.Fatalf("configure api: %v", err)
	}
	proxy := httpadapter.NewV2(apiHandler).ProxyWithContext

	extractor, err := extract.NewClaudeExtractor(ctx, cfg.BedrockRegion, cfg.ExtractModel, recorder)
	if err != nil {
		log.Fatalf("configure extractor: %v", err)
	}
	ingester := ingest.NewHandler(idx, blobs, extractor, embedder, vectorStore)

	adapter := transport.NewTransport(proxy, ingester)
	lambda.Start(adapter.Handle)
}
