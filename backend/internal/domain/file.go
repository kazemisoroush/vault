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
	OwnerID     string            `json:"-" dynamodbav:"ownerId"`
	Key         string            `json:"-" dynamodbav:"key"`
	Name        string            `json:"name" dynamodbav:"name"`
	ContentType string            `json:"contentType" dynamodbav:"contentType"`
	Size        int64             `json:"size" dynamodbav:"size"`
	Status      string            `json:"status" dynamodbav:"status"`
	Meta        map[string]string `json:"meta,omitempty" dynamodbav:"meta,omitempty"`
	Attributes  Attributes        `json:"-" dynamodbav:"attributes,omitempty"`
	CreatedAt   time.Time         `json:"createdAt" dynamodbav:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt" dynamodbav:"updatedAt"`
}

// Attributes are the normalised, queryable facts pulled out of a file's free-form Meta.
// They give structured search a stable set of keys to filter on, next to the raw Meta.
type Attributes struct {
	Person  string `json:"person,omitempty" dynamodbav:"person,omitempty"`
	DocType string `json:"docType,omitempty" dynamodbav:"docType,omitempty"`
	Vendor  string `json:"vendor,omitempty" dynamodbav:"vendor,omitempty"`
	Amount  string `json:"amount,omitempty" dynamodbav:"amount,omitempty"`
	Date    string `json:"date,omitempty" dynamodbav:"date,omitempty"`
}

// attributeKeys lists, for each normalised attribute, the free-form Meta keys that feed it,
// in order of preference. Keys are matched without regard to case or surrounding spaces.
var attributeKeys = map[string][]string{
	"person":  {"person", "name", "patient", "cardholder"},
	"docType": {"document type", "documenttype", "doctype", "type", "category"},
	"vendor":  {"vendor", "merchant", "store", "place", "retailer"},
	"amount":  {"amount", "total", "price", "cost"},
	"date":    {"date", "issued", "purchased"},
}

// AttributesFromMeta derives the normalised attributes from a file's free-form Meta.
// For each attribute it takes the first candidate key that carries a non-empty value.
func AttributesFromMeta(meta map[string]string) Attributes {
	lookup := make(map[string]string, len(meta))
	for key, value := range meta {
		lookup[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	pick := func(attr string) string {
		for _, candidate := range attributeKeys[attr] {
			if value := lookup[candidate]; value != "" {
				return value
			}
		}
		return ""
	}
	return Attributes{
		Person:  pick("person"),
		DocType: pick("docType"),
		Vendor:  pick("vendor"),
		Amount:  pick("amount"),
		Date:    pick("date"),
	}
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
