package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLoadReadsEnvironment checks every field is read from its variable.
func TestLoadReadsEnvironment(t *testing.T) {
	// Arrange
	t.Setenv("VAULT_TABLE", "table-x")
	t.Setenv("VAULT_BUCKET", "bucket-y")
	t.Setenv("VAULT_JWT_ISSUER", "https://issuer.example")
	t.Setenv("VAULT_JWT_CLIENT_ID", "client-123")
	t.Setenv("VAULT_ADDR", ":9090")

	// Act
	cfg := Load()

	// Assert
	assert.Equal(t, "table-x", cfg.Table)
	assert.Equal(t, "bucket-y", cfg.Bucket)
	assert.Equal(t, "https://issuer.example", cfg.JWTIssuer)
	assert.Equal(t, "client-123", cfg.JWTClientID)
	assert.Equal(t, ":9090", cfg.Addr)
}

// TestServerAddrFallsBackToDefault checks the local server port default.
func TestServerAddrFallsBackToDefault(t *testing.T) {
	// Arrange
	t.Setenv("VAULT_ADDR", "")

	// Act
	cfg := Load()

	// Assert
	assert.Equal(t, ":8080", cfg.ServerAddr())
}

// TestAuthDisabledReadsOptOutFlag checks the explicit auth opt-out.
func TestAuthDisabledReadsOptOutFlag(t *testing.T) {
	// Arrange
	t.Setenv("VAULT_AUTH_DISABLED", "true")

	// Act & Assert
	assert.True(t, Load().AuthDisabled)

	// Arrange
	t.Setenv("VAULT_AUTH_DISABLED", "")

	// Act & Assert
	assert.False(t, Load().AuthDisabled)
}

// TestAuthEnabledRequiresBothIssuerAndClient checks the auth toggle.
func TestAuthEnabledRequiresBothIssuerAndClient(t *testing.T) {
	// Arrange
	full := Config{JWTIssuer: "https://issuer.example", JWTClientID: "client-123"}
	missing := Config{JWTIssuer: "https://issuer.example"}

	// Act & Assert
	assert.True(t, full.AuthEnabled())
	assert.False(t, missing.AuthEnabled())
}
