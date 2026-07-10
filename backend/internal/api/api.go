package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/kazemisoroush/vault/backend/internal/agent"
	"github.com/kazemisoroush/vault/backend/internal/api/controller"
	"github.com/kazemisoroush/vault/backend/internal/api/middleware"
	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/blob"
	"github.com/kazemisoroush/vault/backend/internal/config"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/telemetry"
	"github.com/kazemisoroush/vault/backend/internal/vectors"
)

// New builds the API handler: controllers behind the router, wrapped in middleware.
func New(ctx context.Context, cfg config.Config, idx index.Index, blobs blob.Store, store vectors.Store, answerer agent.Answerer, calls controller.CallLister, emitter telemetry.Emitter) (http.Handler, error) {
	router := NewRouter(
		controller.NewFileController(idx, blobs, store),
		controller.NewAskController(answerer, blobs),
		controller.NewCallsController(calls),
		controller.NewHealthController(),
		controller.NewMetricsController(emitter),
	)

	authed, err := authenticate(ctx, cfg, router)
	if err != nil {
		return nil, fmt.Errorf("configure authentication: %w", err)
	}
	// Metrics wraps outermost so it records every request, including auth rejections and recovered panics.
	return middleware.NewMetricsMiddleware(emitter).Wrap(middleware.NewRecoverMiddleware().Wrap(authed)), nil
}

// authenticate wraps the router with JWT auth, failing closed unless opted out.
func authenticate(ctx context.Context, cfg config.Config, routes http.Handler) (http.Handler, error) {
	if cfg.AuthDisabled {
		log.Print("auth explicitly disabled via VAULT_AUTH_DISABLED; serving without authentication")
		return routes, nil
	}
	if !cfg.AuthEnabled() {
		return nil, errors.New("auth not configured: set VAULT_JWT_ISSUER and VAULT_JWT_CLIENT_ID, or set VAULT_AUTH_DISABLED=true to run without auth")
	}

	keyFunc, err := auth.NewCognitoKeyFunc(ctx, cfg.JWTIssuer)
	if err != nil {
		return nil, fmt.Errorf("build auth key resolver: %w", err)
	}
	verifier := auth.NewVerifier(cfg.JWTIssuer, cfg.JWTClientID, keyFunc)
	return middleware.NewAuthMiddleware(verifier).Wrap(routes), nil
}
