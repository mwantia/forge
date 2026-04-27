package session

import (
	"context"
	"fmt"
)

func constructSessionPrefix(sessionID string) string {
	return fmt.Sprintf("sessions/%s/", sessionID)
}

func constructSessionKey(sessionID string) string {
	return constructSessionPrefix(sessionID) + "session.json"
}

func constructMessagePrefix(sessionID string) string {
	return constructSessionPrefix(sessionID) + "messages/"
}

func constructMessageKey(sessionID string, msg *Message) string {
	return fmt.Sprintf("%s%020d.json", constructMessagePrefix(sessionID), msg.CreatedAt.UnixNano())
}

// WithCallerSession threads the dispatching session ID through the context so
// that tool implementations can resolve "the current session" without an
// explicit argument from the LLM.
func WithCallerSession(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, callerSessionKey, sessionID)
}

// CallerSessionID returns the session ID stored by WithCallerSession, or "".
func CallerSessionID(ctx context.Context) string {
	v, _ := ctx.Value(callerSessionKey).(string)
	return v
}

// resolveSessionArg picks a session ID from the tool arguments, falling back to
// the caller session threaded through the context.
func ResolveSessionArg(ctx context.Context, args map[string]any, key string) (string, error) {
	if raw, ok := args[key].(string); ok && raw != "" {
		return raw, nil
	}
	if id := CallerSessionID(ctx); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("missing argument %q and no caller session in context", key)
}

func ArgString(args map[string]any, key string) (string, bool) {
	v, ok := args[key].(string)
	return v, ok && v != ""
}

func ArgInt(args map[string]any, key string, def int) int {
	switch v := args[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return def
}
