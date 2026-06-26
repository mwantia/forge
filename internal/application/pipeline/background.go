package pipeline

import (
	"context"
	"fmt"
	"strings"

	dompipeline "github.com/mwantia/forge/internal/domain/pipeline"
)

type BackgroundDispatcher = dompipeline.BackgroundDispatcher

// DispatchBackground implements BackgroundDispatcher.
//
// It performs the shared pipeline setup (history load, system init, resource
// recall, tool catalog) then launches the run in a goroutine and returns
// immediately. All persistence happens inside the goroutine via the normal
// pipeline path.
//
// ctx is only used for the setup phase; the pipeline goroutine runs under
// context.Background() so it outlives the caller's context.
func (s *PipelineService) DispatchBackground(ctx context.Context, sessionID, ref, content string) error {
	meta, err := s.sessions.ResolveSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("resolve session: %w", err)
	}

	policy := s.config.Output.ResolveOutputPolicy()
	run, err := s.preparePipelineRun(ctx, meta, ref, content, "", policy, true)
	if err != nil {
		return err
	}

	out := make(chan PipelineEvent, 32)
	bgCtx := context.Background()
	go func() {
		if err := s.RunSessionPipeline(bgCtx, run.sess, out); err != nil {
			s.logger.Error("background pipeline error", "session", meta.ID, "ref", ref, "error", err)
		}
	}()
	go func() {
		for range out {
		}
	}()

	return nil
}

// DispatchSync implements BackgroundDispatcher.
//
// Runs the pipeline to completion within ctx, drains all events, and returns
// the accumulated assistant text. The caller blocks until the pipeline is done.
func (s *PipelineService) DispatchSync(ctx context.Context, sessionID, ref, content string) (string, error) {
	meta, err := s.sessions.ResolveSession(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("resolve session: %w", err)
	}

	policy := s.config.Output.ResolveOutputPolicy()
	run, err := s.preparePipelineRun(ctx, meta, ref, content, "", policy, true)
	if err != nil {
		return "", err
	}

	out := make(chan PipelineEvent, 32)
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.RunSessionPipeline(ctx, run.sess, out)
	}()

	var buf strings.Builder
	for ev := range out {
		if chunk, ok := ev.(ChunkEvent); ok {
			buf.WriteString(chunk.Text)
		}
	}

	if err := <-errCh; err != nil {
		return "", err
	}
	return buf.String(), nil
}

var _ BackgroundDispatcher = (*PipelineService)(nil)
