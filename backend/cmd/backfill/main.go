// Backfill embeds every existing file so files stored before vector search became searchable.
package main

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/kazemisoroush/vault/backend/internal/calls"
	appconfig "github.com/kazemisoroush/vault/backend/internal/config"
	"github.com/kazemisoroush/vault/backend/internal/embed"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/vectors"
)

// pageSize is how many records to read per index page.
const pageSize = int32(100)

func main() {
	ctx := context.Background()
	cfg := appconfig.Load()

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("load AWS config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	idx := index.NewDynamoIndex(dynamoClient, cfg.Table)
	recorder := calls.NewDynamoCalls(dynamoClient, cfg.CallsTable)

	embedder, err := embed.NewTitanEmbedder(ctx, cfg.BedrockRegion, cfg.EmbedModel, recorder)
	if err != nil {
		log.Fatalf("configure embedder: %v", err)
	}
	store, err := vectors.NewS3Vectors(ctx, cfg.BedrockRegion, cfg.VectorBucket, cfg.VectorIndex)
	if err != nil {
		log.Fatalf("configure vector store: %v", err)
	}

	total := 0
	for cursor := ""; ; {
		files, next, err := idx.List(ctx, "", pageSize, cursor)
		if err != nil {
			log.Fatalf("list files: %v", err)
		}
		for _, file := range files {
			vector, err := embedder.Embed(ctx, file.SearchText())
			if err != nil {
				log.Printf("embed %s: %v", file.ID, err)
				continue
			}
			if err := store.Put(ctx, file.ID, vector); err != nil {
				log.Printf("store vector for %s: %v", file.ID, err)
				continue
			}
			total++
		}
		if next == "" {
			break
		}
		cursor = next
	}

	log.Printf("backfilled %d files", total)
}
