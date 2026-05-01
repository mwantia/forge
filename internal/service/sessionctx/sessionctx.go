// Package sessionctx carries the dispatching session ID across packages
// without forcing a dependency on internal/service/session. It exists so
// leaf services (resource, etc.) can resolve "the current session" without
// importing the full session package — which itself depends on them.
package sessionctx

import "context"

type ctxKey struct{}

var key = ctxKey{}

// With threads sessionID through ctx.
func With(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, key, sessionID)
}

// From returns the session ID previously placed by With, or "" if none.
func From(ctx context.Context) string {
	v, _ := ctx.Value(key).(string)
	return v
}
