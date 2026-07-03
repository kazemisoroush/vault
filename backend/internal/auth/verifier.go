// Package auth verifies Cognito JWT access tokens.
package auth

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// ErrUnauthorized is returned when a token is missing, malformed, or invalid.
var ErrUnauthorized = errors.New("unauthorized")

const accessTokenUse = "access"

// Verifier validates Cognito access tokens against an issuer and app client.
type Verifier struct {
	issuer   string
	clientID string
	keyFunc  jwt.Keyfunc
}

// NewVerifier builds a Verifier that resolves signing keys via keyFunc.
func NewVerifier(issuer, clientID string, keyFunc jwt.Keyfunc) *Verifier {
	return &Verifier{issuer: issuer, clientID: clientID, keyFunc: keyFunc}
}

// Verify returns nil when the token is a valid access token for this client.
func (v *Verifier) Verify(token string) error {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, v.keyFunc,
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnauthorized, err)
	}
	if claims.TokenUse != accessTokenUse {
		return fmt.Errorf("%w: token_use %q is not %q", ErrUnauthorized, claims.TokenUse, accessTokenUse)
	}
	if claims.ClientID != v.clientID {
		return fmt.Errorf("%w: client_id does not match", ErrUnauthorized)
	}
	return nil
}
