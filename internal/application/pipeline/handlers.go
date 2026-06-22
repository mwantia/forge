package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	appsession "github.com/mwantia/forge/internal/application/session"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
	infratemplate "github.com/mwantia/forge/internal/infrastructure/template"
)

type commitRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Content   string `json:"content"    binding:"required"`
	Mode      string `json:"mode,omitempty"`
}

// handleCommit godoc
//
//	@Description	Appends a user message to the given session and streams the pipeline response as NDJSON. Each line is a WireEvent JSON object.
func (s *PipelineService) handleCommit() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req commitRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()

		meta, err := s.sessions.ResolveSession(ctx, req.SessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		// Resolve target ref. ?fork_from=<msg-hash-or-prefix> auto-creates
		// a new branch off that message's parent and commits there. ?ref=
		// commits on an existing non-HEAD branch.
		ref, err := s.resolveCommitRef(ctx, meta.ID, c.Query("ref"), c.Query("fork_from"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Resolve mode: request.mode > session.Mode > "chat".
		// request.mode is ephemeral — it affects only this turn and is not persisted.
		resolvedMode := appsession.ModeOrDefault(meta.Mode)
		if req.Mode != "" {
			resolvedMode = req.Mode
		}
		if resolvedMode != meta.Mode {
			metaCopy := *meta
			metaCopy.Mode = resolvedMode
			meta = &metaCopy
		}

		output := s.config.Output.resolve()
		if raw, _ := strconv.ParseBool(c.Query("raw")); raw {
			output = rawOverride()
		}

		run, err := s.preparePipelineRun(ctx, meta, ref, req.Content, output)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Writer.Header().Set("X-Forge-Ref", ref)
		c.Writer.Header().Set("X-Forge-Mode", resolvedMode)

		start := time.Now()
		out := make(chan PipelineEvent, 32)
		go func() {
			if err := s.RunSessionPipeline(ctx, run.sess, out); err != nil {
				s.logger.Error("Pipeline error", "session", meta.ID, "error", err)
			}
		}()

		PipelineMessagesTotal.WithLabelValues("commited").Inc()

		// Stream NDJSON response.
		c.Writer.Header().Set("Content-Type", "application/x-ndjson")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			s.logger.Error("ResponseWriter does not support flushing")
			return
		}

		for ev := range out {
			wire, err := ToWireEvent(ev)
			if err != nil {
				s.logger.Error("Failed to convert pipeline event", "error", err)
				continue
			}

			b, _ := json.Marshal(wire)
			c.Writer.Write(append(b, '\n'))
			flusher.Flush()
		}

		go s.sessions.AccumulateDuration(ctx, meta.ID, time.Since(start).Milliseconds())
	}
}

type renderRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Content   string `json:"content"    binding:"required"`
}

type renderResponse struct {
	Content string `json:"content"`
}

// handleRender godoc
//
//	@Description	Renders a template string through the session's scoped template engine (session vars, tool data, model data). Nothing is persisted and no LLM call is made.
func (s *PipelineService) handleRender() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req renderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()

		meta, err := s.sessions.ResolveSession(ctx, req.SessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		scoped, err := s.buildScopedTemplate(ctx, meta)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "template init: " + err.Error()})
			return
		}

		rendered, err := scoped.RenderBody(req.Content)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, renderResponse{Content: rendered})
	}
}

type previewRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Content   string `json:"content"`
}

// previewUsage is the per-fragment size summary returned by /preview. Token
// counts are a heuristic (≈ runes / 4) — accuracy is roughly ±20% across
// common LLM tokenizers. Use for relative comparison, not for billing.
type previewUsage struct {
	Bytes     int `json:"bytes"`
	Runes     int `json:"runes"`
	EstTokens int `json:"est_tokens"`
}

type previewResponseMessage struct {
	Role    string       `json:"role"`
	Content string       `json:"content,omitempty"`
	Usage   previewUsage `json:"usage"`
}

type previewResponse struct {
	SessionID   string                   `json:"session_id"`
	System      string                   `json:"system"`
	SystemUsage previewUsage             `json:"system_usage"`
	Messages    []previewResponseMessage `json:"messages"`
	Total       previewUsage             `json:"total"`
	ToolCount   int                      `json:"tool_count"`
	EstAccuracy string                   `json:"est_accuracy"`
}

// estimateUsage applies the chars-per-token heuristic. Counts UTF-8 runes
// rather than bytes so multi-byte content (CJK, emoji) doesn't inflate the
// estimate. Truth is tokenizer-specific; this is a deliberate ≈ ±20% proxy.
func estimateUsage(text string) previewUsage {
	runes := utf8.RuneCountInString(text)
	return previewUsage{
		Bytes:     len(text),
		Runes:     runes,
		EstTokens: (runes + 3) / 4,
	}
}

func sumUsage(a, b previewUsage) previewUsage {
	return previewUsage{
		Bytes:     a.Bytes + b.Bytes,
		Runes:     a.Runes + b.Runes,
		EstTokens: a.EstTokens + b.EstTokens,
	}
}

// handlePreview godoc
//
//	@Description	Returns the assembled system prompt and chat-message slice that would be sent to the provider, without persisting the new user message or calling the LLM. Use to debug prompt composition.
func (s *PipelineService) handlePreview() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req previewRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()

		meta, err := s.sessions.ResolveSession(ctx, req.SessionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		history, err := s.sessions.ListMessages(ctx, meta.ID, 0, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load message history: " + err.Error()})
			return
		}

		scoped, err := s.buildScopedTemplate(ctx, meta)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "template init: " + err.Error()})
			return
		}

		// Render the full history — system is at history[0] (or absent for a
		// session that has never been committed; preview still works, just empty).
		chatMessages := buildChatMessages(history, scoped, s.logger)

		if req.Content != "" {
			rendered := req.Content
			if r, err := scoped.RenderBody(req.Content); err == nil {
				rendered = r
			}
			chatMessages = append(chatMessages, sdkplugins.ChatMessage{
				Role:    "user",
				Content: rendered,
			})
		}

		toolCalls, err := s.tools.GetAllToolCalls()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve tools: " + err.Error()})
			return
		}
		toolCalls = filterToolCallsByPlugins(toolCalls, builtinNamespaceSetFromRegistar(s.tools), meta.Plugins)

		resp := previewResponse{
			SessionID:   meta.ID,
			Messages:    make([]previewResponseMessage, 0, len(chatMessages)),
			ToolCount:   len(toolCalls),
			EstAccuracy: "±20%",
		}

		for _, m := range chatMessages {
			usage := estimateUsage(m.Content)
			if m.Role == "system" {
				resp.System = m.Content
				resp.SystemUsage = usage
				resp.Total = sumUsage(resp.Total, usage)
				continue
			}
			resp.Messages = append(resp.Messages, previewResponseMessage{
				Role:    m.Role,
				Content: m.Content,
				Usage:   usage,
			})
			resp.Total = sumUsage(resp.Total, usage)
		}

		c.JSON(http.StatusOK, resp)
	}
}

// resolveCommitRef interprets the ?ref= and ?fork_from= commit query
// params (docs/03 §3.2). Precedence: fork_from beats ref. fork_from auto-
// creates a branch named "fork-<8hex>[-N]" off the parent of the named
// message. ref must already exist when supplied.
func (s *PipelineService) resolveCommitRef(ctx context.Context, sessionID, ref, forkFrom string) (string, error) {
	if forkFrom != "" {
		full, err := s.sessions.ResolveMessageHash(ctx, sessionID, forkFrom)
		if err != nil {
			return "", fmt.Errorf("fork_from: %w", err)
		}

		obj, err := s.sessions.GetMessageObj(ctx, full)
		if err != nil {
			return "", fmt.Errorf("fork_from load: %w", err)
		}

		base := "fork-" + full[:8]
		name := base
		for i := 2; ; i++ {
			err := s.sessions.CASRef(ctx, sessionID, name, "", obj.ParentHash)
			if err == nil {
				break
			}

			var conflict *dag.CASConflict
			if errors.As(err, &conflict) {
				name = fmt.Sprintf("%s-%d", base, i)
				continue
			}

			return "", fmt.Errorf("failed to create fork ref: %w", err)
		}

		return name, nil
	}

	if ref == "" || ref == dag.HEAD {
		return dag.HEAD, nil
	}

	hash, err := s.sessions.ReadRef(ctx, sessionID, ref)
	if err != nil {
		return "", err
	}

	if hash == "" {
		return "", fmt.Errorf("ref %q does not exist", ref)
	}

	return ref, nil
}

// recordPromptContext builds a dag.PromptContext snapshot of what the
// provider is about to receive and stores it in the global object pool.
// Returns the resulting hash, which is stamped onto every assistant + tool
// message produced during the run. history must include the system message.
func (s *PipelineService) recordPromptContext(ctx context.Context, meta *appsession.SessionMetadata, history []*appsession.Message, userHash string, tools []sdkplugins.ToolCall) (string, error) {
	provider, model, _ := s.splitModelName(meta.Model)

	hashes := make([]string, 0, len(history)+1)
	for _, m := range history {
		if m.Hash != "" {
			hashes = append(hashes, m.Hash)
		}
	}
	if userHash != "" {
		hashes = append(hashes, userHash)
	}

	catalogHash, err := s.sessions.PutToolCatalog(ctx, buildToolCatalog(tools))
	if err != nil {
		s.logger.Warn("failed to store tool catalog", "error", err)
		catalogHash = ""
	}

	pc := &dag.PromptContext{
		Provider:        provider,
		Model:           model,
		MessageHashes:   hashes,
		ToolCatalogHash: catalogHash,
	}

	return s.sessions.PutPromptContext(ctx, pc)
}

func buildToolCatalog(tools []sdkplugins.ToolCall) *dag.ToolCatalog {
	defs := make([]dag.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		var schema map[string]any
		if b, err := json.Marshal(t.Parameters); err == nil {
			_ = json.Unmarshal(b, &schema)
		}
		defs = append(defs, dag.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Schema:      schema,
		})
	}
	return &dag.ToolCatalog{Tools: defs}
}

// fetchModelSystem fetches the system prompt string from the provider's model
// config. Returns "" on any error.
func (s *PipelineService) fetchModelSystem(ctx context.Context, modelRef string) string {
	providerName, modelName, ok := s.splitModelName(modelRef)
	if !ok {
		return ""
	}

	m, err := s.provider.GetModel(ctx, providerName, modelName)
	if err != nil {
		return ""
	}

	return m.System
}

// buildChatMessages assembles the ChatMessage slice sent to the LLM from the
// persisted history. Every message's Content is rendered through tmpl so
// template expressions resolve to their current values. On render failure the
// raw source is used and the error is logged — the commit is never aborted
// due to a template error in a stored message.
func buildChatMessages(history []*appsession.Message, tmpl *infratemplate.Template, logger hclog.Logger) []sdkplugins.ChatMessage {
	messages := make([]sdkplugins.ChatMessage, 0, len(history))
	for _, msg := range history {
		content := msg.Content

		if rendered, err := tmpl.RenderBody(msg.Content); err != nil {
			logger.Warn("template render failed for stored message", "role", msg.Role, "hash", msg.Hash, "error", err)
		} else {
			content = rendered
		}

		cm := sdkplugins.ChatMessage{
			Role:    msg.Role,
			Content: content,
		}

		switch msg.Role {
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				calls := make([]sdkplugins.ChatToolCall, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					calls = append(calls, sdkplugins.ChatToolCall{
						ID:        tc.ID,
						Name:      tc.Name,
						Arguments: tc.Arguments,
					})
				}
				cm.ToolCalls = &sdkplugins.ChatMessageToolCalls{ToolCalls: calls}
			}
		case "tool":
			if len(msg.ToolCalls) > 0 {
				cm.ToolCalls = &sdkplugins.ChatMessageToolCalls{
					ID:   msg.ToolCalls[0].ID,
					Name: msg.ToolCalls[0].Name,
				}
			}
		}
		messages = append(messages, cm)
	}
	return messages
}
