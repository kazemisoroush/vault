// Lambda entrypoint behind an API Gateway HTTP API (payload format 2.0).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	appconfig "github.com/kazemisoroush/vault/backend/internal/config"
	"github.com/kazemisoroush/vault/backend/internal/handler"
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
	h := handler.New(idx, blobs)

	routes, err := guard(ctx, cfg, h.Routes())
	if err != nil {
		log.Fatalf("configure auth: %v", err)
	}

	lambda.Start(httpadapter.NewV2(routes).ProxyWithContext)
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
