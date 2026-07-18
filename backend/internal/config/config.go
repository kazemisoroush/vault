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
	RerankModel   string
	// KnowledgeBaseID is the managed Bedrock Knowledge Base the searcher queries by hybrid search.
	KnowledgeBaseID string
	// KnowledgeBaseDataSourceID is the Knowledge Base data source the syncer runs ingestion jobs on.
	KnowledgeBaseDataSourceID string
	// FunctionName is this Lambda's own name, set by the Lambda runtime. The check pipeline
	// self-invokes it to run asynchronously; empty outside Lambda.
	FunctionName string
}

// Load reads the configuration from environment variables.
func Load() Config {
	return Config{
		Table:                     os.Getenv("TABLE"),
		CallsTable:                os.Getenv("CALLS_TABLE"),
		ChecksTable:               os.Getenv("CHECKS_TABLE"),
		Bucket:                    os.Getenv("BUCKET"),
		JWTIssuer:                 os.Getenv("JWT_ISSUER"),
		JWTClientID:               os.Getenv("JWT_CLIENT_ID"),
		Addr:                      os.Getenv("ADDR"),
		AuthDisabled:              os.Getenv("AUTH_DISABLED") == "true",
		BedrockRegion:             os.Getenv("BEDROCK_REGION"),
		RerankModel:               os.Getenv("RERANK_MODEL"),
		KnowledgeBaseID:           os.Getenv("KNOWLEDGE_BASE_ID"),
		KnowledgeBaseDataSourceID: os.Getenv("KNOWLEDGE_BASE_DATA_SOURCE_ID"),
		FunctionName:              os.Getenv("AWS_LAMBDA_FUNCTION_NAME"),
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
