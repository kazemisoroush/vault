package auth

import "context"

// ownerIDContextKey is the private key the authenticated owner id is stored under.
type ownerIDContextKey struct{}

// WithOwnerID returns a context carrying the authenticated owner id, the caller's Cognito sub.
func WithOwnerID(ctx context.Context, ownerID string) context.Context {
	return context.WithValue(ctx, ownerIDContextKey{}, ownerID)
}

// OwnerID returns the authenticated owner id from the context, or an empty string when there is none.
func OwnerID(ctx context.Context) string {
	ownerID, _ := ctx.Value(ownerIDContextKey{}).(string)
	return ownerID
}
