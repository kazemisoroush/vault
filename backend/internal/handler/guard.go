package handler

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/kazemisoroush/vault/backend/internal/auth"
	"github.com/kazemisoroush/vault/backend/internal/config"
)

// Guard wraps routes with JWT auth, failing closed unless auth is opted out.
func Guard(ctx context.Context, cfg config.Config, routes http.Handler) (http.Handler, error) {
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
	return RequireAuth(routes, verifier), nil
}
