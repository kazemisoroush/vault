package auth

import "github.com/golang-jwt/jwt/v5"

// Claims are the Cognito access-token claims Vault checks.
type Claims struct {
	ClientID string `json:"client_id"`
	TokenUse string `json:"token_use"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}
