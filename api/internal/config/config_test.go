package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_WithAllEnvVars(t *testing.T) {
	// Arrange
	t.Setenv("DYNAMODB_TABLE", "test-table")
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
	t.Setenv("GOOGLE_REFRESH_TOKEN", "test-refresh-token")
	t.Setenv("OWNER_EMAIL", "owner@example.com")

	// Act
	cfg, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "test-table", cfg.DynamoDBTable)
	assert.Equal(t, "test-client-id", cfg.GoogleClientID)
	assert.Equal(t, "test-client-secret", cfg.GoogleClientSecret)
	assert.Equal(t, "test-refresh-token", cfg.GoogleRefreshToken)
	assert.Equal(t, "owner@example.com", cfg.OwnerEmail)
}

func TestLoad_DefaultDynamoDBTable(t *testing.T) {
	// Arrange
	require.NoError(t, os.Unsetenv("DYNAMODB_TABLE"))
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_REFRESH_TOKEN", "token")
	t.Setenv("OWNER_EMAIL", "owner@example.com")

	// Act
	cfg, err := Load()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "vault-files", cfg.DynamoDBTable)
}

func TestLoad_MissingGoogleClientID(t *testing.T) {
	// Arrange
	require.NoError(t, os.Unsetenv("GOOGLE_CLIENT_ID"))
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_REFRESH_TOKEN", "token")

	// Act
	_, err := Load()

	// Assert
	require.Error(t, err)
}

func TestLoad_MissingGoogleClientSecret(t *testing.T) {
	// Arrange
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	require.NoError(t, os.Unsetenv("GOOGLE_CLIENT_SECRET"))
	t.Setenv("GOOGLE_REFRESH_TOKEN", "token")

	// Act
	_, err := Load()

	// Assert
	require.Error(t, err)
}

func TestLoad_MissingGoogleRefreshToken(t *testing.T) {
	// Arrange
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	require.NoError(t, os.Unsetenv("GOOGLE_REFRESH_TOKEN"))

	// Act
	_, err := Load()

	// Assert
	require.Error(t, err)
}

func TestLoad_MissingOwnerEmail(t *testing.T) {
	// Arrange
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_REFRESH_TOKEN", "token")
	require.NoError(t, os.Unsetenv("OWNER_EMAIL"))

	// Act
	_, err := Load()

	// Assert
	require.Error(t, err)
}
