package auth

import (
	"context"
	"fmt"

	"google.golang.org/api/idtoken"
)

// GoogleValidator validates Google-issued ID tokens for a given audience.
type GoogleValidator struct {
	audience string
}

// NewGoogleValidator creates a validator that accepts tokens for the audience.
func NewGoogleValidator(audience string) *GoogleValidator {
	return &GoogleValidator{audience: audience}
}

// Validate verifies the token's signature, issuer, audience, and expiry.
func (v *GoogleValidator) Validate(ctx context.Context, token string) (string, error) {
	payload, err := idtoken.Validate(ctx, token, v.audience)
	if err != nil {
		return "", fmt.Errorf("validating google id token: %w", err)
	}

	email, _ := payload.Claims["email"].(string)
	if email == "" {
		return "", fmt.Errorf("validating google id token: missing email claim")
	}

	if verified, _ := payload.Claims["email_verified"].(bool); !verified {
		return "", fmt.Errorf("validating google id token: email not verified")
	}

	return email, nil
}
