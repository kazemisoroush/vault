// Package domain holds the core Vault records.
package domain

import "time"

// Status values describe where a file is in the extraction lifecycle.
const (
	StatusPending = "pending"
	StatusReady   = "ready"
	StatusFailed  = "failed"
)

// File is one stored blob and its free-form metadata.
type File struct {
	ID          string            `json:"id" dynamodbav:"id"`
	Key         string            `json:"-" dynamodbav:"key"`
	Name        string            `json:"name" dynamodbav:"name"`
	ContentType string            `json:"contentType" dynamodbav:"contentType"`
	Size        int64             `json:"size" dynamodbav:"size"`
	Status      string            `json:"status" dynamodbav:"status"`
	Meta        map[string]string `json:"meta,omitempty" dynamodbav:"meta,omitempty"`
	CreatedAt   time.Time         `json:"createdAt" dynamodbav:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt" dynamodbav:"updatedAt"`
}
