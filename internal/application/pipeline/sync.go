package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mwantia/forge-sdk/pkg/plugin/provider"
	appsession "github.com/mwantia/forge/internal/application/session"
	domprovider "github.com/mwantia/forge/internal/domain/provider"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
	infratemplate "github.com/mwantia/forge/internal/infrastructure/template"
)

// pipelineRun holds the resolved session and user-message hash produced by
// preparePipelineRun. Callers consume sess via RunSessionPipeline.
type pipelineRun struct {
	sess     *Session
	userHash string
}

// RenderContent implements PipelineRenderer. It resolves the session, builds a
// scoped template, and renders content through it. Returns the raw source on error.
func (s *PipelineService) RenderContent(ctx context.Context, sessionID, content string) (string, error) {
	meta, err := s.sessions.ResolveSession(ctx, sessionID)
	if err != nil {
		return content, err
	}
	scoped, err := s.buildScopedTemplate(ctx, meta, "")
	if err != nil {
		return content, err
	}
	return scoped.RenderBody(content)
}

var _ PipelineRenderer = (*PipelineService)(nil)

// buildScopedTemplate clones the base template and injects session variables,
// the live tool namespace tree, the active model's config data, and the
// request-scoped language. language is a BCP 47 tag; empty resolves to "en".
func (s *PipelineService) buildScopedTemplate(ctx context.Context, meta *appsession.SessionMetadata, language string) (*infratemplate.Template, error) {
	lang, _ := appsession.FindLanguage(language)
	return s.tmpl.Clone(
		appsession.SessionVars(meta),
		infratemplate.WithAnyValue("tools", buildToolsData(ctx, s.tools, meta.Plugins)),
		infratemplate.WithAnyValue("model", buildModelData(ctx, s.provider, s.splitModelName, meta.Model)),
		infratemplate.WithAnyValue("language", map[string]any{
			"code": lang.Code,
			"name": lang.Name,
		}),
	)
}

// buildModelData fetches the active model's config from the provider and
// returns it as a map[string]any for injection as the .model template variable.
// On any error an empty map is returned so the commit is never blocked.
func buildModelData(ctx context.Context, provider domprovider.ProviderRegistar, splitFn func(string) (string, string, bool), modelRef string) map[string]any {
	providerName, modelName, ok := splitFn(modelRef)
	if !ok {
		return map[string]any{}
	}

	m, err := provider.GetModel(ctx, providerName, modelName)
	if err != nil || m == nil {
		return map[string]any{}
	}

	return map[string]any{
		"name":        modelName,
		"provider":    providerName,
		"system":      m.System,
		"temperature": m.Temperature,
	}
}

// preparePipelineRun is the shared setup for all pipeline entry points. It:
//  1. Loads message history along ref.
//  2. Ensures a role=system message exists at history[0], storing the
//     agent-level default on the very first commit if none is present yet.
//  3. Builds a scoped template (session vars + live tool data).
//  4. Renders the full history (including system) via buildChatMessages.
//  5. Persists the user message as raw template source and appends its
//     rendered form to the chat slice.
//  6. Resolves and filters available tool calls.
//  7. Records the PromptContext and stamps its hash.
func (s *PipelineService) preparePipelineRun(ctx context.Context, meta *appsession.SessionMetadata, ref, content, language string, output resolvedOutput) (*pipelineRun, error) {
	var history []*appsession.Message
	var err error
	if ref == dag.HEAD {
		history, err = s.sessions.ListMessages(ctx, meta.ID, 0, 0)
	} else {
		history, err = s.sessions.ListMessagesFromRef(ctx, meta.ID, ref, 0, 0)
	}
	if err != nil {
		return nil, fmt.Errorf("load history: %w", err)
	}

	// On the first commit there is no system message yet. Store the agent-level
	// default as raw template source so it becomes history[0] for this and all
	// future commits.
	if len(history) == 0 || history[0].Role != "system" {
		systemSrc := s.config.System
		if systemSrc == "" {
			systemSrc = DefaultSystemTemplate
		}
		sysMsg := &appsession.Message{
			Role:      "system",
			Content:   systemSrc,
			CreatedAt: time.Now(),
		}
		sysHash, err := s.sessions.AppendMessageToRef(ctx, meta.ID, ref, sysMsg)
		if err != nil {
			s.logger.Warn("failed to store system message in DAG", "session", meta.ID, "error", err)
		}
		sysMsg.Hash = sysHash
		history = append([]*appsession.Message{sysMsg}, history...)
	}

	scoped, err := s.buildScopedTemplate(ctx, meta, language)
	if err != nil {
		return nil, fmt.Errorf("build template: %w", err)
	}

	// Render the full history — system at [0], conversation after.
	chatMessages := buildChatMessages(history, scoped, s.logger)

	// Store user message as raw template source, then append its rendered form.
	userMsg := &appsession.Message{
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	}
	userHash, err := s.sessions.AppendMessageToRef(ctx, meta.ID, ref, userMsg)
	if err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	renderedContent := content
	if r, err := scoped.RenderBody(content); err == nil {
		renderedContent = r
	}
	chatMessages = append(chatMessages, provider.ChatMessage{
		Role:    "user",
		Content: renderedContent,
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

// CommitSync implements session.CommitDispatcher. Runs the full pipeline turn
// synchronously and returns the concatenated assistant response text.
func (s *PipelineService) CommitSync(ctx context.Context, sessionID, ref, content string) (string, error) {
	meta, err := s.sessions.ResolveSession(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("resolve session: %w", err)
	}

	if ref == "" {
		ref = dag.HEAD
	}

	run, err := s.preparePipelineRun(ctx, meta, ref, content, "", s.config.Output.resolve())
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

// CommitEvents implements PipelineCommitter. Returns typed PipelineEvents directly,
// avoiding the WireEvent marshal/unmarshal round-trip needed by the SSE HTML bridge.
// mode overrides the session's stored mode for this turn only; empty string uses session mode.
// language is the BCP 47 response language for this turn; empty resolves to "en".
func (s *PipelineService) CommitEvents(ctx context.Context, sessionID, ref, content, mode, language string) (<-chan PipelineEvent, error) {
	meta, err := s.sessions.ResolveSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("resolve session: %w", err)
	}
	if ref == "" {
		ref = dag.HEAD
	}

	resolvedMode := appsession.ModeOrDefault(meta.Mode)
	if mode != "" {
		resolvedMode = mode
	}
	if resolvedMode != meta.Mode {
		copy := *meta
		copy.Mode = resolvedMode
		meta = &copy
	}

	run, err := s.preparePipelineRun(ctx, meta, ref, content, language, s.config.Output.resolve())
	if err != nil {
		return nil, err
	}
	out := make(chan PipelineEvent, 32)
	go func() {
		if err := s.RunSessionPipeline(ctx, run.sess, out); err != nil {
			s.logger.Error("commit_events pipeline error", "session", meta.ID, "error", err)
		}
	}()
	return out, nil
}

var _ PipelineCommitter = (*PipelineService)(nil)
