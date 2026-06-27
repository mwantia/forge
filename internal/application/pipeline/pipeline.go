package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/mwantia/forge-sdk/pkg/plugin/base"
	"github.com/mwantia/forge-sdk/pkg/plugin/provider"
	appsession "github.com/mwantia/forge/internal/application/session"
	domapprovals "github.com/mwantia/forge/internal/domain/approvals"
	domresource "github.com/mwantia/forge/internal/domain/resource"
)

// PipelineCommitter is the narrow interface used by the UI service to initiate
// a streaming pipeline commit without depending on the concrete PipelineService.
type PipelineCommitter interface {
	// CommitEvents starts a pipeline turn and returns a channel of typed events.
	// mode overrides the session's stored mode for this turn only (empty = use session mode).
	// language is the BCP 47 response language for this turn (empty = "en").
	// recall controls whether the automatic recall hint pipeline runs for this turn.
	CommitEvents(ctx context.Context, sessionID, ref, content, mode, language string, recall bool) (<-chan PipelineEvent, error)
}

// PipelineRenderer renders a raw template string through a session's scoped
// template engine (session vars, tool data, model data).
type PipelineRenderer interface {
	RenderContent(ctx context.Context, sessionID, content string) (string, error)
}

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

	start := time.Now()

	providerName, modelName, _ := s.splitModelName(sess.Metadata.Model)

	if providerName == "" {
		s.logger.Debug("Pipeline start", "session", sess.SessionID, "agent", modelName, "ref", sess.Ref)
	} else {
		s.logger.Debug("Pipeline start", "session", sess.SessionID, "provider", providerName, "model", modelName, "ref", sess.Ref)
	}

	messages := make([]provider.ChatMessage, len(sess.Messages))
	copy(messages, sess.Messages)

	for i := range s.config.MaxToolIterations {
		s.logger.Trace("Pipeline iteration", "iteration", i+1, "session", sess.SessionID, "messages", len(messages))

		content, toolCalls, finalChunk, err := s.chatWithRetry(ctx, providerName, modelName, messages, sess.ToolCalls, out, sess.Policy, i+1)
		if err != nil {
			out <- ErrorEvent{Message: fmt.Sprintf("provider error (iteration %d): %s", i+1, err)}
			return fmt.Errorf("provider chat error (iteration %d): %w", i+1, err)
		}

		s.logger.Debug("Pipeline iteration completed", "iteration", i+1, "session", sess.SessionID, "content_len", len(content), "tool_calls", len(toolCalls))

		if len(toolCalls) == 0 {
			msg := &appsession.Message{
				Role:        "assistant",
				Content:     content,
				CreatedAt:   time.Now(),
				ContextHash: sess.ContextHash,
				Usage:       finalChunk.Usage,
			}

			if err := s.persistMessage(ctx, sess, msg); err != nil {
				out <- ErrorEvent{Message: fmt.Sprintf("storage error (iteration %d): %s", i+1, err)}
				return err
			}

			out <- DoneEvent{
				Usage:    finalChunk.Usage,
				Metadata: finalChunk.Metadata,
				Duration: time.Since(start).Milliseconds(),
			}
			return nil
		}

		// Persist assistant turn with tool calls and emit ToolCallEvents.
		assistantMsg := &appsession.Message{
			Role:        "assistant",
			Content:     content,
			CreatedAt:   time.Now(),
			ContextHash: sess.ContextHash,
			Usage:       finalChunk.Usage,
		}

		for _, tc := range toolCalls {
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, appsession.MessageToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
			out <- ToolCallEvent{
				CallID: tc.ID,
				Name:   tc.Name,
				Args:   tc.Arguments,
			}
		}
		if err := s.persistMessage(ctx, sess, assistantMsg); err != nil {
			out <- ErrorEvent{Message: fmt.Sprintf("storage error (iteration %d): %s", i+1, err)}
			return err
		}

		messages = append(messages, provider.ChatMessage{
			Role:    "assistant",
			Content: content,
			ToolCalls: &provider.ChatMessageToolCalls{
				ToolCalls: toolCalls,
			},
		})

		// Execute all tool calls in parallel.
		type toolResult struct {
			tc       provider.ChatToolCall
			result   any
			isError  bool
			resource *ToolResultResource
		}

		results := make([]toolResult, len(toolCalls))
		var wg sync.WaitGroup
		ctx := appsession.WithCallerSession(ctx, sess.SessionID)

		for idx, tc := range toolCalls {
			wg.Add(1)

			go func(idx int, tc provider.ChatToolCall) {
				defer wg.Done()

				result, isError, resource := s.executeToolCall(ctx, tc)
				results[idx] = toolResult{
					tc:       tc,
					result:   result,
					isError:  isError,
					resource: resource,
				}

			}(idx, tc)
		}
		wg.Wait()

		for _, r := range results {
			out <- ToolResultEvent{
				CallID:   r.tc.ID,
				Name:     r.tc.Name,
				Result:   r.result,
				IsError:  r.isError,
				Resource: r.resource,
			}

			res := marshalResult(r.result)
			msg := &appsession.Message{
				Role:    "tool",
				Content: res,
				ToolCalls: []appsession.MessageToolCall{{
					ID:      r.tc.ID,
					Name:    r.tc.Name,
					Result:  res,
					IsError: r.isError,
				}},
				CreatedAt:   time.Now(),
				ContextHash: sess.ContextHash,
			}

			if err := s.persistMessage(ctx, sess, msg); err != nil {
				out <- ErrorEvent{Message: fmt.Sprintf("storage error (iteration %d): %s", i+1, err)}
				return err
			}

			messages = append(messages, provider.ChatMessage{
				Role:    "tool",
				Content: res,
				ToolCalls: &provider.ChatMessageToolCalls{
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
func (s *PipelineService) streamFromProvider(ctx context.Context, stream provider.ChatStream, out chan<- PipelineEvent, policy ResolveOutputPolicy) (content string, toolCalls []provider.ChatToolCall, final *provider.ChatChunk, err error) {
	defer stream.Close()

	ch := newChunker(out, policy)
	var buf strings.Builder
	var calls []provider.ChatToolCall

	for {
		chunk, recvErr := stream.Recv()
		if recvErr == io.EOF {
			if flushErr := ch.flush(ctx); flushErr != nil {
				return "", nil, nil, flushErr
			}
			return buf.String(), calls, &provider.ChatChunk{Done: true}, nil
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
func (s *PipelineService) chatWithRetry(ctx context.Context, providerName, modelName string, messages []provider.ChatMessage, tools []provider.ToolCall, out chan<- PipelineEvent, policy ResolveOutputPolicy, iteration int) (string, []provider.ChatToolCall, *provider.ChatChunk, error) {
	//
	attempts := s.config.Retry.GetAttempts()

	base, err := s.config.Retry.Backoff.GetBase()
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to resolve backoff config: %w", err)
	}

	max, err := s.config.Retry.Backoff.GetMax()
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to resolve backoff config: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", nil, nil, err
		}

		if providerName == "" {
			s.logger.Debug("Chat dispatch", "iteration", iteration, "attempt", attempt, "agent", modelName, "messages", len(messages), "tools", len(tools))
		} else {
			s.logger.Debug("Chat dispatch", "iteration", iteration, "attempt", attempt, "provider", providerName, "model", modelName, "messages", len(messages), "tools", len(tools))
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

			s.logger.Warn("stream error before any output; will retry", "iteration", iteration, "attempt", attempt, "error", streamErr)
		}

		if attempt == attempts {
			break
		}

		backoff := min(base*(1<<(attempt-1)), max)
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
func (s *PipelineService) executeToolCall(ctx context.Context, tc provider.ChatToolCall) (any, bool, *ToolResultResource) {
	parts := strings.SplitN(tc.Name, "__", 2)
	if len(parts) != 2 {
		s.logger.Warn("Invalid tool call name format", "tool", tc.Name)
		return fmt.Sprintf("error: invalid tool name format %q", tc.Name), true, nil
	}

	namespace, name := strings.ToLower(parts[0]), strings.ToLower(parts[1])
	fullName := tc.Name

	if s.approvals != nil {
		if s.approvals.CheckDeny(fullName) {
			return fmt.Sprintf("error: tool %q is not permitted", fullName), true, nil
		}

		if !s.approvals.CheckAllow(fullName) {
			def, err := s.tools.GetToolDefinition(namespace, name)

			if err == nil && (def.Annotations.RequiresConfirmation || def.Annotations.Destructive) {
				req := base.ApprovalRequest{
					Type:    base.ApprovalTypeToolCall,
					Title:   fullName,
					Message: "Tool requires approval before execution.",
					Details: tc.Arguments,
				}

				meta := domapprovals.ApprovalMeta{
					Plugin:   namespace,
					ToolName: name,
					ToolArgs: tc.Arguments,
				}

				rec, createErr := s.approvals.Create(ctx, req, meta)
				if createErr != nil || rec.Status != domapprovals.StatusAllowed {
					reason := "denied"

					if rec != nil {
						reason = string(rec.Status)
					}

					return fmt.Sprintf("error: tool call %q was %s", fullName, reason), true, nil
				}
			}
		}
	}

	resp, err := s.tools.ExecuteToolWithCallID(ctx, namespace, name, tc.Arguments, tc.ID)
	if err != nil {
		s.logger.Warn("Tool execution error", "tool", tc.Name, "error", err)
		return fmt.Sprintf("error: %v", err), true, nil
	}

	if !resp.Success {
		msg := fmt.Sprintf("error: tool %q failed", fullName)
		if resp.Error != nil {
			msg = resp.Error.Message
		}
		return msg, true, nil
	}

	// Auto-store resource if the plugin requested it.
	var stored *ToolResultResource
	if resp.Resource != nil {
		content := marshalResult(resp.Data)
		res, storeErr := s.resources.Store(ctx, content, "auto-stored by "+name, domresource.ResourceMeta{
			Name:        resp.Resource.Name,
			Type:        resp.Resource.Type,
			Tags:        resp.Resource.Tags,
			Description: resp.Resource.Description,
		})
		if storeErr != nil {
			s.logger.Warn("Failed to auto-store tool resource", "tool", fullName, "error", storeErr)
		} else {
			stored = &ToolResultResource{
				ID:          res.ID,
				Name:        res.Meta.Name,
				Type:        res.Meta.Type,
				Tags:        res.Meta.Tags,
				Description: res.Meta.Description,
			}
		}
	}

	return resp.Data, false, stored
}

// persistMessage saves a message to storage via the session manager.
// On success the new content-hash is set on msg.Hash and msg.ParentHash
// is filled in by the store (the existing HEAD before this write).
func (s *PipelineService) persistMessage(ctx context.Context, sess *Session, msg *appsession.Message) error {
	_, err := s.sessions.AppendMessageToRef(ctx, sess.SessionID, sess.Ref, msg)
	if err != nil {
		s.logger.Error("Failed to persist message", "session", sess.SessionID, "role", msg.Role, "error", err)
		return fmt.Errorf("failed to persist message: %w", err)
	}
	return nil
}

// marshalResult converts a tool result to a string for storage and LLM feed-back.
func marshalResult(v any) string {
	if v == nil {
		return "null"
	}

	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}

	return string(b)
}

// splitModelName splits "provider/model" into its parts. Bare names (no slash)
// are treated as agent names and returned with an empty provider component.
// The bool return is always true; it exists to satisfy the buildModelData
// function signature without breaking callers.
func (s *PipelineService) splitModelName(modelName string) (string, string, bool) {
	if p, m, ok := strings.Cut(modelName, "/"); ok {
		return strings.ToLower(p), m, true
	}
	return "", modelName, true
}

var _ PipelineExecutor = (*PipelineService)(nil)
