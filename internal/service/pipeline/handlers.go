package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/session"
	"github.com/mwantia/forge/internal/service/session/dag"
	"github.com/mwantia/forge/internal/service/tools"
)

type dispatchRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Content   string `json:"content"    binding:"required"`
	NoStore   bool   `json:"no_store"`
}

// handleDispatch godoc
//
//	@Summary		Dispatch message
//	@Description	Appends a user message to the given session and streams the pipeline response as NDJSON. Each line is a WireEvent JSON object.
//	@Tags			pipeline
//	@Accept			json
//	@Produce		application/x-ndjson
//	@Param			body	body		dispatchRequest		true	"Dispatch request"
//	@Success		200		{object}	WireEvent			"NDJSON stream; one event per line"
//	@Failure		400		{object}	map[string]string
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/dispatch [post]
func (s *PipelineService) handleDispatch() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dispatchRequest
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
		// a new branch off that message's parent and dispatches there. ?ref=
		// dispatches on an existing non-HEAD branch.
		ref, err := s.resolveDispatchRef(ctx, meta.ID, c.Query("ref"), c.Query("fork_from"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Load full message history along the target ref.
		var history []*session.Message
		if ref == dag.HEAD {
			history, err = s.sessions.ListMessages(ctx, meta.ID, 0, 0)
		} else {
			history, err = s.sessions.ListMessagesFromRef(ctx, meta.ID, ref, 0, 0)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load message history: " + err.Error()})
			return
		}

		// If there's no system message at the root yet, assemble and persist one
		// before the first user message is committed (lazy init for sessions
		// created without an explicit regen call).
		if len(history) == 0 || history[0].Role != "system" {
			history, err = s.initSystemMessage(ctx, meta, ref, history)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to init system message: " + err.Error()})
				return
			}
		}

		// Append per-turn resources block to system content without persisting it.
		chatMessages := buildChatMessages(history)
		if resources := s.recallRelevantResources(ctx, meta.ID, req.Content); resources != "" && len(chatMessages) > 0 && chatMessages[0].Role == "system" {
			chatMessages[0].Content += "\n\n" + resources
		}

		// Persist user message before pipeline start (skipped when no_store is set).
		userMsg := &session.Message{
			Role:      "user",
			Content:   req.Content,
			CreatedAt: time.Now(),
		}
		var userHash string
		if !req.NoStore {
			h, err := s.sessions.AppendMessageToRef(ctx, meta.ID, ref, userMsg)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save message: " + err.Error()})
				return
			}
			userHash = h
		}

		chatMessages = append(chatMessages, sdkplugins.ChatMessage{
			Role:    "user",
			Content: req.Content,
		})

		// Resolve all tool calls (full namespace__name form for the LLM).
		toolCalls, err := s.tools.GetAllToolCalls()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve tools: " + err.Error()})
			return
		}
		toolCalls = filterToolCallsByPlugins(toolCalls, builtinNamespaceSetFromRegistar(s.tools), meta.Plugins)

		output := s.config.Output.resolve()
		if raw, _ := strconv.ParseBool(c.Query("raw")); raw {
			output = rawOverride()
		}

		// Materialize PromptContext for this dispatch (docs/03 §1.2). The
		// hash is stamped onto every assistant + tool message produced
		// during the run so the turn is replayable later.
		ctxHash, err := s.recordPromptContext(ctx, meta, history, userHash, toolCalls)
		if err != nil {
			s.logger.Warn("failed to record prompt context", "session", meta.ID, "error", err)
		}

		sess := &Session{
			SessionID:   meta.ID,
			Metadata:    meta,
			Messages:    chatMessages,
			ToolCalls:   toolCalls,
			NoStore:     req.NoStore,
			Ref:         ref,
			ContextHash: ctxHash,
			Output:      output,
		}

		c.Writer.Header().Set("X-Forge-Ref", ref)

		out := make(chan PipelineEvent, 32)
		go func() {
			if err := s.RunSessionPipeline(ctx, sess, out); err != nil {
				s.logger.Error("Pipeline error", "session", meta.ID, "error", err)
			}
		}()

		PipelineMessagesTotal.WithLabelValues("dispatched").Inc()

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
			b, _ := json.MarshalIndent(wire, "", "  ")
			c.Writer.Write(append(b, '\n'))
			flusher.Flush()
		}
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
//	@Summary		Preview pipeline input
//	@Description	Returns the assembled system prompt and chat-message slice that would be sent to the provider, without persisting the new user message or calling the LLM. Use to debug prompt composition.
//	@Tags			pipeline
//	@Accept			json
//	@Produce		json
//	@Param			body	body		previewRequest	true	"Preview request"
//	@Success		200		{object}	previewResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/preview [post]
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

		chatMessages := buildChatMessages(history)
		if resources := s.recallRelevantResources(ctx, meta.ID, req.Content); resources != "" && len(chatMessages) > 0 && chatMessages[0].Role == "system" {
			chatMessages[0].Content += "\n\n" + resources
		}
		if req.Content != "" {
			chatMessages = append(chatMessages, sdkplugins.ChatMessage{
				Role:    "user",
				Content: req.Content,
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

// recallRelevantResources queries the resource service for items that
// match the user's input on the session's namespace and renders them as
// the <relevant-resources> prompt block. Returns "" on any error or when
// the resource registar is unbound — the dispatch must never break
// because of a memory layer failure.
func (s *PipelineService) recallRelevantResources(ctx context.Context, sessionID, query string) string {
	if s.resources == nil || strings.TrimSpace(query) == "" {
		return ""
	}
	const resourceRecallLimit = 5
	hits, err := s.resources.Recall(ctx, sdkplugins.RecallQuery{
		Path:  "/sessions/" + sessionID,
		Query: query,
		Limit: resourceRecallLimit,
	})
	if err != nil {
		s.logger.Debug("resource recall failed", "session", sessionID, "error", err)
		return ""
	}
	if len(hits) == 0 {
		return ""
	}
	items := make([]resourceItem, 0, len(hits))
	for _, h := range hits {
		items = append(items, resourceItem{ID: h.ID, Content: h.Content})
	}
	return renderResourcesBlock(items)
}

// resolveDispatchRef interprets the ?ref= and ?fork_from= dispatch query
// params (docs/03 §3.2). Precedence: fork_from beats ref. fork_from auto-
// creates a branch named "fork-<8hex>[-N]" off the parent of the named
// message. ref must already exist when supplied.
func (s *PipelineService) resolveDispatchRef(ctx context.Context, sessionID, ref, forkFrom string) (string, error) {
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
			existing, err := s.sessions.ReadRef(ctx, sessionID, name)
			if err != nil {
				return "", err
			}
			if existing == "" {
				break
			}
			name = fmt.Sprintf("%s-%d", base, i)
		}
		if err := s.sessions.WriteRef(ctx, sessionID, name, obj.ParentHash); err != nil {
			return "", fmt.Errorf("create fork ref: %w", err)
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
func (s *PipelineService) recordPromptContext(
	ctx context.Context,
	meta *session.SessionMetadata,
	history []*session.Message,
	userHash string,
	tools []sdkplugins.ToolCall,
) (string, error) {
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

	_ = tools

	pc := &dag.PromptContext{
		Provider:      provider,
		Model:         model,
		MessageHashes: hashes,
	}
	return s.sessions.PutPromptContext(ctx, pc)
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

// initSystemMessage assembles and persists a system MessageObj as the root of
// the target ref when none exists yet. It appends the system message to
// history and returns the updated slice.
func (s *PipelineService) initSystemMessage(ctx context.Context, meta *session.SessionMetadata, ref string, history []*session.Message) ([]*session.Message, error) {
	scoped, err := s.tmpl.Clone(session.SessionVars(meta))
	if err != nil {
		return history, err
	}
	modelSystem := s.fetchModelSystem(ctx, meta.Model)
	agentSystem := s.config.System
	if agentSystem == "" {
		agentSystem = DefaultAgentSystem
	}
	layers := collectPromptLayers(ctx, agentSystem, modelSystem, meta, s.tools, s.logger)
	content := assembleSystemPrompt(layers, scoped, s.logger)

	sysMsg := &session.Message{Role: "system", Content: content}
	if _, err := s.sessions.AppendMessageToRef(ctx, meta.ID, ref, sysMsg); err != nil {
		return history, err
	}
	return append([]*session.Message{sysMsg}, history...), nil
}

// builtinNamespaceSetFromRegistar returns a set of builtin namespace names
// directly from the tools registar, avoiding dependency on promptLayers.
func builtinNamespaceSetFromRegistar(r tools.ToolsRegistar) map[string]struct{} {
	ns := r.ListNamespaces()
	set := make(map[string]struct{}, len(ns))
	for _, n := range ns {
		if n.Builtin {
			set[strings.ToLower(n.Namespace)] = struct{}{}
		}
	}
	return set
}

// buildChatMessages assembles the ChatMessage slice sent to the LLM from
// the persisted history. The system message (role="system") is expected to be
// history[0] when present; all roles pass through unchanged.
func buildChatMessages(history []*session.Message) []sdkplugins.ChatMessage {
	var messages []sdkplugins.ChatMessage
	for _, msg := range history {
		cm := sdkplugins.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
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
