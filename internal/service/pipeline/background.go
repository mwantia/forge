package pipeline

import (
	"context"
	"fmt"
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/session"
	"github.com/mwantia/forge/internal/service/session/dag"
)

// BackgroundDispatcher runs a pipeline turn asynchronously without holding
// an HTTP connection open. EventService uses this to fire webhook-triggered
// pipelines after creating the target branch.
type BackgroundDispatcher interface {
	DispatchBackground(ctx context.Context, sessionID, ref, content string) error
}

// DispatchBackground implements BackgroundDispatcher.
//
// It performs the same setup as handleDispatch (history load, system init,
// resource recall, tool catalog) but returns as soon as the goroutine is
// launched. All persistence (user message, assistant messages, prompt context)
// happens inside the goroutine via the normal pipeline path.
//
// ctx is only used for the setup phase; the pipeline goroutine runs under
// context.Background() so it outlives the caller's context.
func (s *PipelineService) DispatchBackground(ctx context.Context, sessionID, ref, content string) error {
	meta, err := s.sessions.ResolveSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("resolve session: %w", err)
	}

	var history []*session.Message
	if ref == dag.HEAD {
		history, err = s.sessions.ListMessages(ctx, meta.ID, 0, 0)
	} else {
		history, err = s.sessions.ListMessagesFromRef(ctx, meta.ID, ref, 0, 0)
	}
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}

	if len(history) == 0 || history[0].Role != "system" {
		history, err = s.initSystemMessage(ctx, meta, ref, history)
		if err != nil {
			return fmt.Errorf("init system message: %w", err)
		}
	}

	chatMessages := buildChatMessages(history)
	if resources := s.recallRelevantResources(ctx, meta.ID, content); resources != "" && len(chatMessages) > 0 && chatMessages[0].Role == "system" {
		chatMessages[0].Content += "\n\n" + resources
	}

	userMsg := &session.Message{
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	}
	userHash, err := s.sessions.AppendMessageToRef(ctx, meta.ID, ref, userMsg)
	if err != nil {
		return fmt.Errorf("save user message: %w", err)
	}

	chatMessages = append(chatMessages, sdkplugins.ChatMessage{
		Role:    "user",
		Content: content,
	})

	toolCalls, err := s.tools.GetAllToolCalls()
	if err != nil {
		return fmt.Errorf("resolve tools: %w", err)
	}
	toolCalls = filterToolCallsByPlugins(toolCalls, builtinNamespaceSetFromRegistar(s.tools), meta.Plugins)

	ctxHash, err := s.recordPromptContext(ctx, meta, history, userHash, toolCalls)
	if err != nil {
		s.logger.Warn("failed to record prompt context", "session", meta.ID, "error", err)
	}

	sess := &Session{
		SessionID:   meta.ID,
		Metadata:    meta,
		Messages:    chatMessages,
		ToolCalls:   toolCalls,
		Ref:         ref,
		ContextHash: ctxHash,
		Output:      s.config.Output.resolve(),
	}

	go func() {
		out := make(chan PipelineEvent, 32)
		bgCtx := context.Background()
		go func() {
			if err := s.RunSessionPipeline(bgCtx, sess, out); err != nil {
				s.logger.Error("background pipeline error", "session", meta.ID, "ref", ref, "error", err)
			}
		}()
		for range out {} // drain until RunSessionPipeline closes out
	}()

	return nil
}

var _ BackgroundDispatcher = (*PipelineService)(nil)
