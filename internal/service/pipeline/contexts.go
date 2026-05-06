package pipeline

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/session/dag"
)

// materializedMessage is the resolved shape returned by /materialized.
type materializedMessage struct {
	Hash      string                `json:"hash"`
	Role      string                `json:"role"`
	Content   string                `json:"content,omitempty"`
	ToolCalls []dag.MessageToolCall `json:"tool_calls,omitempty"`
}

type materializedResponse struct {
	Hash            string                `json:"hash"`
	Provider        string                `json:"provider"`
	Model           string                `json:"model"`
	ToolCatalogHash string                `json:"tool_catalog_hash,omitempty"`
	Options         map[string]any        `json:"options,omitempty"`
	Messages        []materializedMessage `json:"messages"`
}

// handleGetContext godoc
//
//	@Summary		Get prompt context
//	@Description	Returns the raw PromptContext blob for a hash. The materialized form is at GET /v1/contexts/{hash}/materialized.
//	@Tags			contexts
//	@Produce		json
//	@Param			hash	path		string	true	"PromptContext hash"
//	@Success		200		{object}	map[string]any
//	@Failure		404		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/contexts/{hash} [get]
func (s *PipelineService) handleGetContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		hash := c.Param("hash")
		pc, err := s.sessions.GetPromptContext(c.Request.Context(), hash)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"hash": hash, "context": pc})
	}
}

// handleMaterializeContext godoc
//
//	@Summary		Materialize prompt context
//	@Description	Resolves a PromptContext's referenced message hashes into a fully expanded chat slice.
//	@Tags			contexts
//	@Produce		json
//	@Param			hash	path		string	true	"PromptContext hash"
//	@Success		200		{object}	map[string]any
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/contexts/{hash}/materialized [get]
func (s *PipelineService) handleMaterializeContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		hash := c.Param("hash")

		pc, err := s.sessions.GetPromptContext(ctx, hash)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		resp := materializedResponse{
			Hash:            hash,
			Provider:        pc.Provider,
			Model:           pc.Model,
			ToolCatalogHash: pc.ToolCatalogHash,
			Options:         pc.Options,
			Messages:        make([]materializedMessage, 0, len(pc.MessageHashes)),
		}
		for _, mh := range pc.MessageHashes {
			obj, err := s.sessions.GetMessageObj(ctx, mh)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("resolve %s: %v", mh, err)})
				return
			}
			resp.Messages = append(resp.Messages, materializedMessage{
				Hash:      mh,
				Role:      obj.Role,
				Content:   obj.Content,
				ToolCalls: obj.ToolCalls,
			})
		}

		c.JSON(http.StatusOK, resp)
	}
}

type replayRequest struct {
	Model           string         `json:"model"`
	OptionsOverride map[string]any `json:"options_override"`
}

// handleReplayContext godoc
//
//	@Summary		Replay prompt context
//	@Description	Re-dispatches a stored PromptContext to a provider and streams the response as NDJSON. The original session is not modified; this is purely a debugging surface.
//	@Tags			contexts
//	@Accept			json
//	@Produce		application/x-ndjson
//	@Param			hash	path		string			true	"PromptContext hash"
//	@Param			body	body		replayRequest	false	"Optional model + options override"
//	@Success		200		{object}	WireEvent		"NDJSON stream; one event per line"
//	@Failure		400		{object}	map[string]string
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/contexts/{hash}/replay [post]
func (s *PipelineService) handleReplayContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		hash := c.Param("hash")

		var req replayRequest
		// Body is optional. Ignore decode errors when there is no body.
		_ = c.ShouldBindJSON(&req)

		pc, err := s.sessions.GetPromptContext(ctx, hash)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		providerName, modelName := pc.Provider, pc.Model
		if req.Model != "" {
			if p, m, ok := s.splitModelName(req.Model); ok {
				providerName, modelName = p, m
			}
		}

		messages := make([]sdkplugins.ChatMessage, 0, len(pc.MessageHashes))
		for _, mh := range pc.MessageHashes {
			obj, err := s.sessions.GetMessageObj(ctx, mh)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("resolve %s: %v", mh, err)})
				return
			}
			messages = append(messages, sdkplugins.ChatMessage{
				Role:    obj.Role,
				Content: obj.Content,
			})
		}

		stream, err := s.provider.Chat(ctx, providerName, modelName, messages, nil)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		out := make(chan PipelineEvent, 32)
		policy := s.config.Output.resolve()
		go func() {
			defer close(out)
			content, _, finalChunk, err := s.streamFromProvider(ctx, stream, out, policy)
			if err != nil {
				out <- ErrorEvent{Message: err.Error()}
				return
			}
			done := DoneEvent{}
			if finalChunk != nil {
				done.Usage = finalChunk.Usage
				done.Metadata = finalChunk.Metadata
			}
			_ = content
			out <- done
		}()

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
