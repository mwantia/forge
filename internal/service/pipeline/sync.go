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

// pipelineRun holds the resolved session and user-message hash produced by
// preparePipelineRun. Callers consume sess via RunSessionPipeline.
type pipelineRun struct {
	sess     *Session
	userHash string
}

// preparePipelineRun is the shared setup sequence for all three pipeline entry
// points (handleCommit, CommitSync, DispatchBackground). It:
//  1. Loads message history along ref.
//  2. Initialises the system message if absent.
//  3. Recalls relevant resources and injects them into the system turn.
//  4. Persists the user message and appends it to the chat slice.
//  5. Resolves and filters available tool calls.
//  6. Records the PromptContext and stamps its hash.
//
// The caller is responsible for template rendering and ref/fork resolution
// before calling this helper.
func (s *PipelineService) preparePipelineRun(
	ctx context.Context,
	meta *session.SessionMetadata,
	ref, content string,
	output resolvedOutput,
) (*pipelineRun, error) {
	var history []*session.Message
	var err error
	if ref == dag.HEAD {
		history, err = s.sessions.ListMessages(ctx, meta.ID, 0, 0)
	} else {
		history, err = s.sessions.ListMessagesFromRef(ctx, meta.ID, ref, 0, 0)
	}
	if err != nil {
		return nil, fmt.Errorf("load history: %w", err)
	}

	if len(history) == 0 || history[0].Role != "system" {
		history, err = s.initSystemMessage(ctx, meta, ref, history)
		if err != nil {
			return nil, fmt.Errorf("init system message: %w", err)
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
		return nil, fmt.Errorf("save user message: %w", err)
	}

	chatMessages = append(chatMessages, sdkplugins.ChatMessage{
		Role:    "user",
		Content: content,
	})

	toolCalls, err := s.tools.GetAllToolCalls()
	if err != nil {
		return nil, fmt.Errorf("resolve tools: %w", err)
	}
	toolCalls = filterToolCallsByPlugins(toolCalls, builtinNamespaceSetFromRegistar(s.tools), meta.Plugins)

	ctxHash, err := s.recordPromptContext(ctx, meta, history, userHash, toolCalls)
	if err != nil {
		s.logger.Warn("failed to record prompt context", "session", meta.ID, "error", err)
	}

	return &pipelineRun{
		userHash: userHash,
		sess: &Session{
			SessionID:   meta.ID,
			Metadata:    meta,
			Messages:    chatMessages,
			ToolCalls:   toolCalls,
			Ref:         ref,
			ContextHash: ctxHash,
			Output:      output,
		},
	}, nil
}

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

	run, err := s.preparePipelineRun(ctx, meta, ref, content, s.config.Output.resolve())
	if err != nil {
		return "", err
	}

	out := make(chan PipelineEvent, 32)
	go func() {
		if err := s.RunSessionPipeline(ctx, run.sess, out); err != nil {
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

