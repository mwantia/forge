package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/session"
)

// PipelineExecutor is the interface for running a session pipeline.
type PipelineExecutor interface {
	RunSessionPipeline(ctx context.Context, s *Session, out chan<- PipelineEvent) error
}

// PipelineStream wraps the receive end of a pipeline event channel.
// Transport adapters range over Channel and call ToWireEvent on each event.
type PipelineStream struct {
	Channel <-chan PipelineEvent
}

// RunSessionPipeline runs the full dispatch loop for a session:
//  1. Calls the provider and forwards TokenEvents live (true streaming).
//  2. If the final chunk carries tool calls, emits ToolCallEvent per call,
//     executes them in parallel, emits ToolResultEvent per result, and loops.
//  3. When the response has no tool calls, emits DoneEvent and returns.
//
// The caller must close nothing; RunSessionPipeline always closes out.
func (s *PipelineService) RunSessionPipeline(ctx context.Context, sess *Session, out chan<- PipelineEvent) error {
	defer close(out)

	providerName, modelName, ok := s.splitModelName(sess.Metadata.Model)
	if !ok {
		return fmt.Errorf("invalid model format, expected '<provider>/<model>', got '%s'", sess.Metadata.Model)
	}

	messages := make([]sdkplugins.ChatMessage, len(sess.Messages))
	copy(messages, sess.Messages)

	for i := range s.config.MaxToolIterations {
		s.logger.Trace("Pipeline iteration", "iteration", i+1, "session", sess.SessionID)

		content, toolCalls, finalChunk, err := s.chatWithRetry(ctx, providerName, modelName, messages, sess.ToolCalls, out, sess.Output, i+1)
		if err != nil {
			out <- ErrorEvent{Message: fmt.Sprintf("provider error (iteration %d): %s", i+1, err)}
			return fmt.Errorf("provider chat error (iteration %d): %w", i+1, err)
		}
		s.logger.Debug("Pipeline iteration completed", "iteration", i+1, "session", sess.SessionID, "content_len", len(content), "tool_calls", len(toolCalls))

		if len(toolCalls) == 0 {
			s.persistMessage(ctx, sess, &session.Message{
				Role:        "assistant",
				Content:     content,
				CreatedAt:   time.Now(),
				ContextHash: sess.ContextHash,
				Usage:       finalChunk.Usage,
			})
			out <- DoneEvent{
				Usage:    finalChunk.Usage,
				Metadata: finalChunk.Metadata,
			}
			return nil
		}

		// Persist assistant turn with tool calls and emit ToolCallEvents.
		assistantMsg := &session.Message{
			Role:        "assistant",
			Content:     content,
			CreatedAt:   time.Now(),
			ContextHash: sess.ContextHash,
			Usage:       finalChunk.Usage,
		}
		for _, tc := range toolCalls {
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, session.MessageToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
			out <- ToolCallEvent{CallID: tc.ID, Name: tc.Name, Args: tc.Arguments}
		}
		s.persistMessage(ctx, sess, assistantMsg)

		messages = append(messages, sdkplugins.ChatMessage{
			Role:    finalChunk.Role,
			Content: content,
			ToolCalls: &sdkplugins.ChatMessageToolCalls{
				ToolCalls: toolCalls,
			},
		})

		// Execute all tool calls in parallel.
		type toolResult struct {
			tc      sdkplugins.ChatToolCall
			result  any
			isError bool
		}
		results := make([]toolResult, len(toolCalls))
		var wg sync.WaitGroup
		execCtx := session.WithCallerSession(ctx, sess.SessionID)
		for idx, tc := range toolCalls {
			wg.Add(1)
			go func(idx int, tc sdkplugins.ChatToolCall) {
				defer wg.Done()
				result, isError := s.executeToolCall(execCtx, tc)
				results[idx] = toolResult{tc: tc, result: result, isError: isError}
			}(idx, tc)
		}
		wg.Wait()

		for _, r := range results {
			out <- ToolResultEvent{
				CallID:  r.tc.ID,
				Name:    r.tc.Name,
				Result:  r.result,
				IsError: r.isError,
			}

			resultStr := marshalResult(r.result)
			s.persistMessage(ctx, sess, &session.Message{
				Role:    "tool",
				Content: resultStr,
				ToolCalls: []session.MessageToolCall{{
					ID:      r.tc.ID,
					Name:    r.tc.Name,
					Result:  resultStr,
					IsError: r.isError,
				}},
				CreatedAt:   time.Now(),
				ContextHash: sess.ContextHash,
			})
			messages = append(messages, sdkplugins.ChatMessage{
				Role:    "tool",
				Content: resultStr,
				ToolCalls: &sdkplugins.ChatMessageToolCalls{
					ID:   r.tc.ID,
					Name: r.tc.Name,
				},
			})
		}
	}

	out <- ErrorEvent{Message: fmt.Sprintf("max tool iterations (%d) reached without final response", s.config.MaxToolIterations)}
	return fmt.Errorf("max tool iterations (%d) reached without a final response", s.config.MaxToolIterations)
}

// streamFromProvider reads from a provider ChatStream, feeding deltas through
// a chunker that emits ChunkEvents at the configured boundary. Returns when
// the stream signals Done. finalChunk carries Done=true plus any tool calls.
func (s *PipelineService) streamFromProvider(ctx context.Context, stream sdkplugins.ChatStream, out chan<- PipelineEvent, policy resolvedOutput) (content string, toolCalls []sdkplugins.ChatToolCall, final *sdkplugins.ChatChunk, err error) {
	defer stream.Close()

	ch := newChunker(out, policy)
	var buf strings.Builder
	var calls []sdkplugins.ChatToolCall
	for {
		chunk, recvErr := stream.Recv()
		if recvErr == io.EOF {
			if flushErr := ch.flush(ctx); flushErr != nil {
				return "", nil, nil, flushErr
			}
			return buf.String(), calls, &sdkplugins.ChatChunk{Done: true}, nil
		}
		if recvErr != nil {
			return "", nil, nil, recvErr
		}

		if chunk.Delta != "" || chunk.Thinking != "" {
			buf.WriteString(chunk.Delta)
			if err := ch.push(ctx, chunk.Delta, chunk.Thinking); err != nil {
				return "", nil, nil, err
			}
		}

		if len(chunk.ToolCalls) > 0 {
			calls = append(calls, chunk.ToolCalls...)
		}

		if chunk.Done {
			if flushErr := ch.flush(ctx); flushErr != nil {
				return "", nil, nil, flushErr
			}
			return buf.String(), calls, chunk, nil
		}
	}
}

// chatWithRetry calls the provider and drains its stream, retrying transient
// failures up to providerMaxAttempts times. A retry is only attempted when
// nothing has been committed to the client yet (no content, no tool calls)
// — otherwise replaying the call would duplicate already-streamed tokens.
//
// ctx cancellation aborts immediately; all other errors are treated as
// transient and retried with exponential backoff.
func (s *PipelineService) chatWithRetry(
	ctx context.Context,
	providerName, modelName string,
	messages []sdkplugins.ChatMessage,
	tools []sdkplugins.ToolCall,
	out chan<- PipelineEvent,
	policy resolvedOutput,
	iteration int,
) (string, []sdkplugins.ChatToolCall, *sdkplugins.ChatChunk, error) {
	retry := s.config.Retry.resolve()

	var lastErr error
	for attempt := 1; attempt <= retry.Attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", nil, nil, err
		}

		stream, err := s.provider.Chat(ctx, providerName, modelName, messages, tools)
		if err != nil {
			lastErr = err
			s.logger.Warn("provider chat failed", "iteration", iteration, "attempt", attempt, "error", err)
		} else {
			content, toolCalls, finalChunk, streamErr := s.streamFromProvider(ctx, stream, out, policy)
			if streamErr == nil {
				return content, toolCalls, finalChunk, nil
			}
			lastErr = streamErr
			// Tokens already on the wire — replay would duplicate. Bail.
			if len(content) > 0 || len(toolCalls) > 0 {
				s.logger.Warn("stream error after partial output; not retrying",
					"iteration", iteration, "attempt", attempt, "error", streamErr)
				return "", nil, nil, streamErr
			}
			s.logger.Warn("stream error before any output; will retry",
				"iteration", iteration, "attempt", attempt, "error", streamErr)
		}

		if attempt == retry.Attempts {
			break
		}
		backoff := min(retry.BaseBackoff*(1<<(attempt-1)), retry.MaxBackoff)
		select {
		case <-ctx.Done():
			return "", nil, nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return "", nil, nil, lastErr
}

// executeToolCall dispatches a single tool call via the tools registrar.
// Tool call names are expected in "namespace__name" format.
func (s *PipelineService) executeToolCall(ctx context.Context, tc sdkplugins.ChatToolCall) (any, bool) {
	parts := strings.SplitN(tc.Name, "__", 2)
	if len(parts) != 2 {
		s.logger.Warn("Invalid tool call name format", "tool", tc.Name)
		return fmt.Sprintf("error: invalid tool name format %q", tc.Name), true
	}

	namespace, name := strings.ToLower(parts[0]), strings.ToLower(parts[1])
	resp, err := s.tools.ExecuteToolWithCallID(ctx, namespace, name, tc.Arguments, tc.ID)
	if err != nil {
		s.logger.Warn("Tool execution error", "tool", tc.Name, "error", err)
		return fmt.Sprintf("error: %v", err), true
	}

	return resp.Result, resp.IsError
}

// persistMessage saves a message to storage via the session manager, logging
// on failure (non-fatal). It is a no-op when sess.NoStore is true.
//
// On success the new content-hash is set on msg.Hash and msg.ParentHash
// is filled in by the store (the existing HEAD before this write).
func (s *PipelineService) persistMessage(ctx context.Context, sess *Session, msg *session.Message) {
	if sess.NoStore {
		return
	}
	if _, err := s.sessions.AppendMessageToRef(ctx, sess.SessionID, sess.Ref, msg); err != nil {
		s.logger.Error("Failed to persist message", "session", sess.SessionID, "role", msg.Role, "error", err)
	}
}

// marshalResult converts a tool result to a string for storage and LLM feed-back.
func marshalResult(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func (s *PipelineService) splitModelName(modelName string) (string, string, bool) {
	parts := strings.SplitN(modelName, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.ToLower(parts[0]), strings.ToLower(parts[1]), true
}

var _ PipelineExecutor = (*PipelineService)(nil)
