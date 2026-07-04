// Local development server exposing the same routes as the Lambda.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/kazemisoroush/vault/backend/internal/api"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	appconfig "github.com/kazemisoroush/vault/backend/internal/config"
	"github.com/kazemisoroush/vault/backend/internal/index"
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

	apiHandler, err := api.New(ctx, cfg, idx, blobs)
	if err != nil {
		log.Fatalf("configure api: %v", err)
	}

	log.Printf("vault backend listening on %s", cfg.ServerAddr())
	if err := http.ListenAndServe(cfg.ServerAddr(), apiHandler); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
