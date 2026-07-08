package knowledge

import (
	"strings"

	"github.com/kazemisoroush/vault/backend/internal/domain"
)

// matches reports whether a file passes the filter. Text fields are compared as a
// case-insensitive substring against the file's normalised attributes, and the creation
// time must fall within the Since and Until bounds when they are set.
func (f Filter) matches(file domain.File) bool {
	if !contains(file.Attributes.Person, f.Person) {
		return false
	}
	if !contains(file.Attributes.DocType, f.DocType) {
		return false
	}
	if !contains(file.Attributes.Vendor, f.Vendor) {
		return false
	}
	if !f.Since.IsZero() && file.CreatedAt.Before(f.Since) {
		return false
	}
	if !f.Until.IsZero() && file.CreatedAt.After(f.Until) {
		return false
	}
	return true
}

// contains reports whether want is empty, or appears in have without regard to case.
func contains(have, want string) bool {
	if want == "" {
		return true
	}
	return strings.Contains(strings.ToLower(have), strings.ToLower(want))
}
