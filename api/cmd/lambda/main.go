package main

import (
	"context"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awslambda "github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/kazemisoroush/vault/api/internal/blob"
	"github.com/kazemisoroush/vault/api/internal/config"
	"github.com/kazemisoroush/vault/api/internal/handler"
	"github.com/kazemisoroush/vault/api/internal/metadata"
	"github.com/kazemisoroush/vault/api/internal/storage"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("loading AWS config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	repo := storage.NewDynamoRepository(dynamoClient, cfg.DynamoDBTable)

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{drive.DriveScope},
	}

	token := &oauth2.Token{
		RefreshToken: cfg.GoogleRefreshToken,
	}
	tokenSource := oauthConfig.TokenSource(ctx, token)

	blobStorage, err := blob.NewDriveStorage(ctx, tokenSource)
	if err != nil {
		log.Fatalf("creating blob storage: %v", err)
	}

	metadataService := metadata.NewService(blobStorage, repo)

	mux := http.NewServeMux()
	h := handler.NewHandler(metadataService)
	h.RegisterRoutes(mux)

	adapter := awslambda.New(mux)
	lambda.Start(adapter.ProxyWithContext)
}
