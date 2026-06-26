package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awslambda "github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/kazemisoroush/vault/api/internal/handler"
	"github.com/kazemisoroush/vault/api/internal/metadata"
	"github.com/kazemisoroush/vault/api/internal/storage"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"

	driveClient "github.com/kazemisoroush/vault/api/internal/drive"
)

func main() {
	ctx := context.Background()

	tableName := os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		tableName = "vault-files"
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("loading AWS config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	repo := storage.NewDynamoRepository(dynamoClient, tableName)

	oauthConfig := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Endpoint:     google.Endpoint,
		Scopes:       []string{drive.DriveScope},
	}

	token := &oauth2.Token{
		RefreshToken: os.Getenv("GOOGLE_REFRESH_TOKEN"),
	}
	tokenSource := oauthConfig.TokenSource(ctx, token)

	driveService, err := driveClient.NewClient(ctx, tokenSource)
	if err != nil {
		log.Fatalf("creating drive client: %v", err)
	}

	metadataService := metadata.NewService(driveService, repo)

	mux := http.NewServeMux()
	h := handler.NewHandler(metadataService)
	h.RegisterRoutes(mux)

	adapter := awslambda.New(mux)
	lambda.Start(adapter.ProxyWithContext)
}
