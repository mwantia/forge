package session

import "context"

type callerSessionKey struct{}

var callerKey = callerSessionKey{}

// WithCallerSession threads the dispatching session ID through ctx so that
// tool implementations can resolve "the current session" without an explicit
// argument.
func WithCallerSession(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, callerKey, sessionID)
}

// CallerSessionID returns the session ID previously placed by WithCallerSession,
// or "" if none.
func CallerSessionID(ctx context.Context) string {
	v, _ := ctx.Value(callerKey).(string)
	return v
}
