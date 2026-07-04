// Package middleware holds pre-request and post-request HTTP middleware.
package middleware

//go:generate go tool mockgen -source=verifier.go -destination=../../mocks/verifier_mock.go -package=mocks

// TokenVerifier reports whether a bearer token is valid.
type TokenVerifier interface {
	Verify(token string) error
}
