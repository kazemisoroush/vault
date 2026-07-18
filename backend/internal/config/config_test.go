package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLoadReadsEnvironment checks every field is read from its variable.
func TestLoadReadsEnvironment(t *testing.T) {
	// Arrange
	t.Setenv("VAULT_TABLE", "table-x")
	t.Setenv("VAULT_CALLS_TABLE", "calls-x")
	t.Setenv("VAULT_CHECKS_TABLE", "checks-x")
	t.Setenv("VAULT_BUCKET", "bucket-y")
	t.Setenv("VAULT_JWT_ISSUER", "https://issuer.example")
	t.Setenv("VAULT_JWT_CLIENT_ID", "client-123")
	t.Setenv("VAULT_ADDR", ":9090")
	t.Setenv("VAULT_AUTH_DISABLED", "true")
	t.Setenv("VAULT_BEDROCK_REGION", "us-east-1")
	t.Setenv("VAULT_EXTRACT_MODEL", "extract-model")
	t.Setenv("VAULT_RERANK_MODEL", "rerank-model")
	t.Setenv("VAULT_EMBED_MODEL", "embed-model")
	t.Setenv("VAULT_VECTOR_BUCKET", "vault-vectors")
	t.Setenv("VAULT_VECTOR_INDEX", "files")
	t.Setenv("VAULT_KNOWLEDGE_BASE_ID", "kb-abc123")
	t.Setenv("VAULT_KNOWLEDGE_BASE_DATA_SOURCE_ID", "ds-xyz789")
	t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "vault-fn")

	// Act
	cfg := Load()

	// Assert
	assert.Equal(t, "table-x", cfg.Table)
	assert.Equal(t, "calls-x", cfg.CallsTable)
	assert.Equal(t, "checks-x", cfg.ChecksTable)
	assert.Equal(t, "bucket-y", cfg.Bucket)
	assert.Equal(t, "https://issuer.example", cfg.JWTIssuer)
	assert.Equal(t, "client-123", cfg.JWTClientID)
	assert.Equal(t, ":9090", cfg.Addr)
	assert.True(t, cfg.AuthDisabled)
	assert.Equal(t, "us-east-1", cfg.BedrockRegion)
	assert.Equal(t, "extract-model", cfg.ExtractModel)
	assert.Equal(t, "rerank-model", cfg.RerankModel)
	assert.Equal(t, "embed-model", cfg.EmbedModel)
	assert.Equal(t, "vault-vectors", cfg.VectorBucket)
	assert.Equal(t, "files", cfg.VectorIndex)
	assert.Equal(t, "kb-abc123", cfg.KnowledgeBaseID)
	assert.Equal(t, "ds-xyz789", cfg.KnowledgeBaseDataSourceID)
	assert.Equal(t, "vault-fn", cfg.FunctionName)
}

// TestAuthDisabledIsFalseUnlessExactlyTrue checks the opt-out reads only the literal "true", so a
// stray value never silently turns auth off.
func TestAuthDisabledIsFalseUnlessExactlyTrue(t *testing.T) {
	// Arrange: a truthy-looking but non-exact value.
	t.Setenv("VAULT_AUTH_DISABLED", "TRUE")

	// Act & Assert: only the exact string "true" disables auth.
	assert.False(t, Load().AuthDisabled)
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

// TestServerAddrUsesConfiguredValue checks a set address wins over the default.
func TestServerAddrUsesConfiguredValue(t *testing.T) {
	// Arrange
	t.Setenv("VAULT_ADDR", ":9090")

	// Act
	cfg := Load()

	// Assert
	assert.Equal(t, ":9090", cfg.ServerAddr())
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

// TestAuthEnabledRequiresBothIssuerAndClient checks the auth toggle: it is on only when both the
// issuer and the client id are present, and off if either is missing.
func TestAuthEnabledRequiresBothIssuerAndClient(t *testing.T) {
	// Arrange
	full := Config{JWTIssuer: "https://issuer.example", JWTClientID: "client-123"}
	issuerOnly := Config{JWTIssuer: "https://issuer.example"}
	clientOnly := Config{JWTClientID: "client-123"}
	neither := Config{}

	// Act & Assert
	assert.True(t, full.AuthEnabled())
	assert.False(t, issuerOnly.AuthEnabled())
	assert.False(t, clientOnly.AuthEnabled())
	assert.False(t, neither.AuthEnabled())
}
