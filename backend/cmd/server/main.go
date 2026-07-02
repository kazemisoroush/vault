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

	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/handler"
	"github.com/kazemisoroush/vault/backend/internal/index"
)

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("load AWS config: %v", err)
	}

	idx := index.NewDynamoIndex(dynamodb.NewFromConfig(cfg), os.Getenv("VAULT_TABLE"))
	blobs := blob.NewS3Store(s3.NewFromConfig(cfg), os.Getenv("VAULT_BUCKET"))
	h := handler.New(idx, blobs)

	addr := os.Getenv("VAULT_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("vault backend listening on %s", addr)
	if err := http.ListenAndServe(addr, h.Routes()); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
