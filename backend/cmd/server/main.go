// Local development server exposing the same routes as the Lambda.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/kazemisoroush/vault/backend/internal/agent"
	"github.com/kazemisoroush/vault/backend/internal/api"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/calls"
	"github.com/kazemisoroush/vault/backend/internal/checks"
	appconfig "github.com/kazemisoroush/vault/backend/internal/config"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/kb"
	"github.com/kazemisoroush/vault/backend/internal/llm"
	"github.com/kazemisoroush/vault/backend/internal/telemetry"
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

	// Retrieval runs against the managed Knowledge Base by hybrid search, for the agent and the check.
	searcher := kb.NewBedrockSearcher(bedrockagentruntime.NewFromConfig(awsCfg), cfg.KnowledgeBaseID)

	answerer := agent.NewQuestionAnswerer(llm.NewModel(cfg.BedrockRegion, cfg.RerankModel, agent.ModelOp, recorder), searcher, idx)

	// Locally there is no Lambda to self-invoke, so the check pipeline runs in a goroutine.
	checkStore := checks.NewDynamoChecks(dynamoClient, cfg.ChecksTable)
	checkModel := llm.NewModel(cfg.BedrockRegion, cfg.RerankModel, checks.ModelOp, recorder)
	verifier := checks.NewVerifier(checkStore, searcher, idx, checkModel)
	enqueuer := checks.NewLocalEnqueuer(verifier)

	apiHandler, err := api.NewHandler(ctx, cfg, idx, blobs, answerer, checkStore, enqueuer, recorder, telemetry.NewEMFEmitter(os.Stdout))
	if err != nil {
		log.Fatalf("configure api: %v", err)
	}

	log.Printf("vault backend listening on %s", cfg.ServerAddr())
	if err := http.ListenAndServe(cfg.ServerAddr(), apiHandler); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
