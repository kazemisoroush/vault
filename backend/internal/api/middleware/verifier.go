// Package middleware holds pre-request and post-request HTTP middleware.
package middleware

//go:generate go tool mockgen -source=verifier.go -destination=../../mocks/verifier_mock.go -package=mocks

// TokenVerifier validates a bearer token and returns its subject (the caller's Cognito sub).
type TokenVerifier interface {
	Verify(token string) (string, error)
}
