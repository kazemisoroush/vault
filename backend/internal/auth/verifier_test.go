package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIssuer   = "https://cognito-idp.us-east-1.amazonaws.com/pool-1"
	testClientID = "client-abc"
	testSubject  = "user-sub-123"
)

// sign builds a signed token string from the given claims and key.
func sign(t *testing.T, key *rsa.PrivateKey, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

// validClaims returns a fresh, valid access-token claim set.
func validClaims() Claims {
	return Claims{
		ClientID: testClientID,
		TokenUse: "access",
		Username: "soroush",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    testIssuer,
			Subject:   testSubject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
}

func TestVerify(t *testing.T) {
	// Arrange
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyFunc := func(*jwt.Token) (any, error) { return &key.PublicKey, nil }
	verifier := NewVerifier(testIssuer, testClientID, keyFunc)

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid access token",
			token:   sign(t, key, validClaims()),
			wantErr: false,
		},
		{
			name: "wrong issuer",
			token: sign(t, key, func() Claims {
				c := validClaims()
				c.Issuer = "https://evil.example"
				return c
			}()),
			wantErr: true,
		},
		{
			name: "wrong client id",
			token: sign(t, key, func() Claims {
				c := validClaims()
				c.ClientID = "someone-else"
				return c
			}()),
			wantErr: true,
		},
		{
			name: "id token not access token",
			token: sign(t, key, func() Claims {
				c := validClaims()
				c.TokenUse = "id"
				return c
			}()),
			wantErr: true,
		},
		{
			name: "expired",
			token: sign(t, key, func() Claims {
				c := validClaims()
				c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
				return c
			}()),
			wantErr: true,
		},
		{
			name: "no subject",
			token: sign(t, key, func() Claims {
				c := validClaims()
				c.Subject = ""
				return c
			}()),
			wantErr: true,
		},
		{
			name:    "signed by an unknown key",
			token:   sign(t, otherKey, validClaims()),
			wantErr: true,
		},
		{
			name:    "garbage token",
			token:   "not-a-jwt",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			owner, err := verifier.Verify(tc.token)

			// Assert
			if tc.wantErr {
				assert.ErrorIs(t, err, ErrUnauthorized)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testSubject, owner)
			}
		})
	}
}
