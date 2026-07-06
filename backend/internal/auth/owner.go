package auth

import "context"

// ownerContextKey is the private key the authenticated owner is stored under.
type ownerContextKey struct{}

// WithOwner returns a context carrying the authenticated owner, the caller's Cognito sub.
func WithOwner(ctx context.Context, owner string) context.Context {
	return context.WithValue(ctx, ownerContextKey{}, owner)
}

// Owner returns the authenticated owner from the context, or an empty string when there is none.
func Owner(ctx context.Context) string {
	owner, _ := ctx.Value(ownerContextKey{}).(string)
	return owner
}
