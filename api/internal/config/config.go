package config

import (
	"fmt"
	"os"
)

// Config holds all application configuration.
type Config struct {
	DynamoDBTable      string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRefreshToken string
	OwnerEmail         string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		DynamoDBTable:      getEnvOrDefault("DYNAMODB_TABLE", "vault-files"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRefreshToken: os.Getenv("GOOGLE_REFRESH_TOKEN"),
		OwnerEmail:         os.Getenv("OWNER_EMAIL"),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.GoogleClientID == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}
	if c.GoogleClientSecret == "" {
		return fmt.Errorf("GOOGLE_CLIENT_SECRET is required")
	}
	if c.GoogleRefreshToken == "" {
		return fmt.Errorf("GOOGLE_REFRESH_TOKEN is required")
	}
	if c.OwnerEmail == "" {
		return fmt.Errorf("OWNER_EMAIL is required")
	}
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
