package pipeline

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/session"
	"github.com/mwantia/forge/internal/service/template"
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
//	@Router			/v1/pipeline/dispatch [post]
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

		// Load full message history for LLM context.
		history, err := s.sessions.ListMessages(ctx, meta.ID, 0, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load message history: " + err.Error()})
			return
		}

		// Fetch model system prompt from the provider.
		modelSystem := ""
		providerName, modelName, ok := s.splitModelName(meta.Model)
		if ok {
			if m, err := s.provider.GetModel(ctx, providerName, modelName); err == nil {
				modelSystem = m.System
			}
		}

		// Render system prompts with session-scoped template clone.
		scoped, err := s.tmpl.Clone(session.SessionVars(meta))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build session template: " + err.Error()})
			return
		}

		// Build chat message slice: assembled system prompt + history.
		agentSystem := s.config.System
		if agentSystem == "" {
			agentSystem = DefaultAgentSystem
		}
		layers := collectPromptLayers(ctx, agentSystem, modelSystem, meta, s.tools, s.logger)
		chatMessages := buildChatMessages(scoped, layers, history, s.logger)

		// Persist user message before pipeline start (skipped when no_store is set).
		userMsg := &session.Message{
			ID:        template.GenerateNewID(),
			Role:      "user",
			Content:   req.Content,
			CreatedAt: time.Now(),
		}
		if !req.NoStore {
			if err := s.sessions.AppendMessage(ctx, meta.ID, userMsg); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save message: " + err.Error()})
				return
			}
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

		output := s.config.Output.resolve()
		if raw, _ := strconv.ParseBool(c.Query("raw")); raw {
			output = rawOverride()
		}

		sess := &Session{
			SessionID: meta.ID,
			Metadata:  meta,
			Messages:  chatMessages,
			ToolCalls: toolCalls,
			NoStore:   req.NoStore,
			Output:    output,
		}

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
			b, _ := json.Marshal(wire)
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
	SessionID    string                   `json:"session_id"`
	System       string                   `json:"system"`
	SystemUsage  previewUsage             `json:"system_usage"`
	Messages     []previewResponseMessage `json:"messages"`
	Total        previewUsage             `json:"total"`
	ToolCount    int                      `json:"tool_count"`
	EstAccuracy  string                   `json:"est_accuracy"`
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
//	@Router			/v1/pipeline/preview [post]
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

		modelSystem := ""
		providerName, modelName, ok := s.splitModelName(meta.Model)
		if ok {
			if m, err := s.provider.GetModel(ctx, providerName, modelName); err == nil {
				modelSystem = m.System
			}
		}

		scoped, err := s.tmpl.Clone(session.SessionVars(meta))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build session template: " + err.Error()})
			return
		}

		agentSystem := s.config.System
		if agentSystem == "" {
			agentSystem = DefaultAgentSystem
		}
		layers := collectPromptLayers(ctx, agentSystem, modelSystem, meta, s.tools, s.logger)
		chatMessages := buildChatMessages(scoped, layers, history, s.logger)
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

// buildChatMessages assembles the ChatMessage slice sent to the LLM:
// one assembled system message (if any layer contributed) followed by
// the persisted message history.
//
// All prompt-assembly logic lives in prompt.go; this function only converts
// the rendered string into a ChatMessage and replays history.
func buildChatMessages(tmpl *template.Template, layers promptLayers, history []*session.Message, logger hclog.Logger) []sdkplugins.ChatMessage {
	var messages []sdkplugins.ChatMessage
	if content := assembleSystemPrompt(layers, tmpl, logger); content != "" {
		messages = append(messages, sdkplugins.ChatMessage{
			Role:    "system",
			Content: content,
		})
	}

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
