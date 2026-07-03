package handler

//go:generate go tool mockgen -source=authenticator.go -destination=../mocks/authenticator_mock.go -package=mocks

// TokenVerifier reports whether a bearer token is valid.
type TokenVerifier interface {
	Verify(token string) error
}
