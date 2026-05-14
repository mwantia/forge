package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/session"
	"github.com/mwantia/forge/internal/service/session/dag"
)

// CommitSync implements session.CommitDispatcher. It runs the full pipeline
// turn for sessionID synchronously — same setup as DispatchBackground, but
// blocks until the tool-loop finishes and returns the concatenated assistant
// response text.
func (s *PipelineService) CommitSync(ctx context.Context, sessionID, ref, content string) (string, error) {
	meta, err := s.sessions.ResolveSession(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("resolve session: %w", err)
	}

	if ref == "" {
		ref = dag.HEAD
	}

	if scoped, err := s.tmpl.Clone(session.SessionVars(meta)); err == nil {
		if rendered, err := scoped.RenderBody(content); err == nil {
			content = rendered
		}
	}

	var history []*session.Message
	if ref == dag.HEAD {
		history, err = s.sessions.ListMessages(ctx, meta.ID, 0, 0)
	} else {
		history, err = s.sessions.ListMessagesFromRef(ctx, meta.ID, ref, 0, 0)
	}
	if err != nil {
		return "", fmt.Errorf("load history: %w", err)
	}

	if len(history) == 0 || history[0].Role != "system" {
		history, err = s.initSystemMessage(ctx, meta, ref, history)
		if err != nil {
			return "", fmt.Errorf("init system message: %w", err)
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
		return "", fmt.Errorf("save user message: %w", err)
	}

	chatMessages = append(chatMessages, sdkplugins.ChatMessage{
		Role:    "user",
		Content: content,
	})

	toolCalls, err := s.tools.GetAllToolCalls()
	if err != nil {
		return "", fmt.Errorf("resolve tools: %w", err)
	}
	toolCalls = filterToolCallsByPlugins(toolCalls, builtinNamespaceSetFromRegistar(s.tools), meta.Plugins)

	ctxHash, err := s.recordPromptContext(ctx, meta, history, userHash, toolCalls)
	if err != nil {
		s.logger.Warn("commit_sync: failed to record prompt context", "session", meta.ID, "error", err)
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

	out := make(chan PipelineEvent, 32)
	go func() {
		if err := s.RunSessionPipeline(ctx, sess, out); err != nil {
			s.logger.Error("commit_sync pipeline error", "session", meta.ID, "error", err)
		}
	}()

	var sb strings.Builder
	var pipeErr error
	for ev := range out {
		switch e := ev.(type) {
		case ChunkEvent:
			sb.WriteString(e.Text)
		case ErrorEvent:
			pipeErr = fmt.Errorf("%s", e.Message)
		}
	}
	if pipeErr != nil {
		return "", pipeErr
	}
	return sb.String(), nil
}

