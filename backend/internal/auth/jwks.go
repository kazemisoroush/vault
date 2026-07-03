package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// NewCognitoKeyFunc builds a JWKS-backed key resolver for a Cognito issuer.
func NewCognitoKeyFunc(ctx context.Context, issuer string) (jwt.Keyfunc, error) {
	jwksURL := strings.TrimRight(issuer, "/") + "/.well-known/jwks.json"
	resolver, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("build jwks keyfunc for %s: %w", jwksURL, err)
	}
	return resolver.Keyfunc, nil
}
