// Package domain holds the core Vault records.
package domain

import (
	"sort"
	"strings"
	"time"
)

// Status values describe where a file is in the extraction lifecycle.
const (
	StatusPending = "pending"
	StatusReady   = "ready"
	StatusFailed  = "failed"
)

// File is one stored blob and its free-form metadata.
type File struct {
	ID          string            `json:"id" dynamodbav:"id"`
	Owner       string            `json:"-" dynamodbav:"owner"`
	Key         string            `json:"-" dynamodbav:"key"`
	Name        string            `json:"name" dynamodbav:"name"`
	ContentType string            `json:"contentType" dynamodbav:"contentType"`
	Size        int64             `json:"size" dynamodbav:"size"`
	Status      string            `json:"status" dynamodbav:"status"`
	Meta        map[string]string `json:"meta,omitempty" dynamodbav:"meta,omitempty"`
	CreatedAt   time.Time         `json:"createdAt" dynamodbav:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt" dynamodbav:"updatedAt"`
}

// SearchText is the name and metadata joined into the text that gets embedded for search.
// Keys are sorted so the same file always produces the same text.
func (f File) SearchText() string {
	parts := []string{f.Name}
	keys := make([]string, 0, len(f.Meta))
	for key := range f.Meta {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, key+": "+f.Meta[key])
	}
	return strings.Join(parts, "\n")
}
