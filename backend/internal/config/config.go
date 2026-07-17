// Package config centralises reading runtime settings from the environment.
package config

import "os"

const defaultServerAddr = ":8080"

// Config holds every runtime setting the backend reads from the environment.
type Config struct {
	Table         string
	CallsTable    string
	ChecksTable   string
	Bucket        string
	JWTIssuer     string
	JWTClientID   string
	Addr          string
	AuthDisabled  bool
	BedrockRegion string
	ExtractModel  string
	RerankModel   string
	EmbedModel    string
	VectorBucket  string
	VectorIndex   string
	// KnowledgeBaseID is the managed Bedrock Knowledge Base the searcher queries by hybrid search.
	KnowledgeBaseID string
	// FunctionName is this Lambda's own name, set by the Lambda runtime. The check pipeline
	// self-invokes it to run asynchronously; empty outside Lambda.
	FunctionName string
}

// Load reads the configuration from environment variables.
func Load() Config {
	return Config{
		Table:           os.Getenv("VAULT_TABLE"),
		CallsTable:      os.Getenv("VAULT_CALLS_TABLE"),
		ChecksTable:     os.Getenv("VAULT_CHECKS_TABLE"),
		Bucket:          os.Getenv("VAULT_BUCKET"),
		JWTIssuer:       os.Getenv("VAULT_JWT_ISSUER"),
		JWTClientID:     os.Getenv("VAULT_JWT_CLIENT_ID"),
		Addr:            os.Getenv("VAULT_ADDR"),
		AuthDisabled:    os.Getenv("VAULT_AUTH_DISABLED") == "true",
		BedrockRegion:   os.Getenv("VAULT_BEDROCK_REGION"),
		ExtractModel:    os.Getenv("VAULT_EXTRACT_MODEL"),
		RerankModel:     os.Getenv("VAULT_RERANK_MODEL"),
		EmbedModel:      os.Getenv("VAULT_EMBED_MODEL"),
		VectorBucket:    os.Getenv("VAULT_VECTOR_BUCKET"),
		VectorIndex:     os.Getenv("VAULT_VECTOR_INDEX"),
		KnowledgeBaseID: os.Getenv("VAULT_KNOWLEDGE_BASE_ID"),
		FunctionName:    os.Getenv("AWS_LAMBDA_FUNCTION_NAME"),
	}
}

// ServerAddr returns the local server address, defaulting when unset.
func (c Config) ServerAddr() string {
	if c.Addr == "" {
		return defaultServerAddr
	}
	return c.Addr
}

// AuthEnabled reports whether JWT verification is fully configured.
func (c Config) AuthEnabled() bool {
	return c.JWTIssuer != "" && c.JWTClientID != ""
}
