// Local development server exposing the same routes as the Lambda.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/kazemisoroush/vault/backend/internal/agent"
	"github.com/kazemisoroush/vault/backend/internal/api"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/calls"
	"github.com/kazemisoroush/vault/backend/internal/checks"
	appconfig "github.com/kazemisoroush/vault/backend/internal/config"
	"github.com/kazemisoroush/vault/backend/internal/embed"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/llm"
	"github.com/kazemisoroush/vault/backend/internal/telemetry"
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

	// Locally there is no Lambda to self-invoke, so the check pipeline runs in a goroutine.
	checkStore := checks.NewDynamoChecks(dynamoClient, cfg.ChecksTable)
	checkModel := llm.NewModel(cfg.BedrockRegion, cfg.RerankModel, checks.ModelOp, recorder)
	runner := checks.NewRunner(checkStore, idx, blobs, embedder, vectorStore, checkModel)
	enqueuer := checks.NewLocalEnqueuer(runner)

	apiHandler, err := api.NewHandler(ctx, cfg, idx, blobs, vectorStore, answerer, checkStore, enqueuer, recorder, telemetry.NewEMFEmitter(os.Stdout))
	if err != nil {
		log.Fatalf("configure api: %v", err)
	}

	log.Printf("vault backend listening on %s", cfg.ServerAddr())
	if err := http.ListenAndServe(cfg.ServerAddr(), apiHandler); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
