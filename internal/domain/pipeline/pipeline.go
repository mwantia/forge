package pipeline

import "context"

// BackgroundDispatcher runs a pipeline turn without holding an HTTP connection
// open. EventService uses this to push webhook-triggered pipelines after
// creating the target branch.
type BackgroundDispatcher interface {
	DispatchBackground(ctx context.Context, sessionID, ref, content string) error
	DispatchSync(ctx context.Context, sessionID, ref, content string) (string, error)
}
