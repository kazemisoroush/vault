package config

import (
	"os"
	"testing"
)

func TestLoad_WithAllEnvVars(t *testing.T) {
	// Arrange
	t.Setenv("DYNAMODB_TABLE", "test-table")
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
	t.Setenv("GOOGLE_REFRESH_TOKEN", "test-refresh-token")

	// Act
	cfg, err := Load()

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DynamoDBTable != "test-table" {
		t.Errorf("DynamoDBTable = %q, want %q", cfg.DynamoDBTable, "test-table")
	}
	if cfg.GoogleClientID != "test-client-id" {
		t.Errorf("GoogleClientID = %q, want %q", cfg.GoogleClientID, "test-client-id")
	}
	if cfg.GoogleClientSecret != "test-client-secret" {
		t.Errorf("GoogleClientSecret = %q, want %q", cfg.GoogleClientSecret, "test-client-secret")
	}
	if cfg.GoogleRefreshToken != "test-refresh-token" {
		t.Errorf("GoogleRefreshToken = %q, want %q", cfg.GoogleRefreshToken, "test-refresh-token")
	}
}

func TestLoad_DefaultDynamoDBTable(t *testing.T) {
	// Arrange
	os.Unsetenv("DYNAMODB_TABLE")
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_REFRESH_TOKEN", "token")

	// Act
	cfg, err := Load()

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DynamoDBTable != "vault-files" {
		t.Errorf("DynamoDBTable = %q, want %q", cfg.DynamoDBTable, "vault-files")
	}
}

func TestLoad_MissingGoogleClientID(t *testing.T) {
	// Arrange
	os.Unsetenv("GOOGLE_CLIENT_ID")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_REFRESH_TOKEN", "token")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoad_MissingGoogleClientSecret(t *testing.T) {
	// Arrange
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	os.Unsetenv("GOOGLE_CLIENT_SECRET")
	t.Setenv("GOOGLE_REFRESH_TOKEN", "token")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoad_MissingGoogleRefreshToken(t *testing.T) {
	// Arrange
	t.Setenv("GOOGLE_CLIENT_ID", "id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	os.Unsetenv("GOOGLE_REFRESH_TOKEN")

	// Act
	_, err := Load()

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
