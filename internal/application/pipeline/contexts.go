package pipeline

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge-sdk/pkg/plugin/provider"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
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
//	@Description	Returns the raw PromptContext blob for a hash. The materialized form is at GET /v1/contexts/{hash}/materialized.
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
//	@Description	Resolves a PromptContext's referenced message hashes into a fully expanded chat slice.
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

			content := obj.Content
			if rendered, err := s.tmpl.RenderBody(obj.Content); err == nil {
				content = rendered
			}

			resp.Messages = append(resp.Messages, materializedMessage{
				Hash:      mh,
				Role:      obj.Role,
				Content:   content,
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
//	@Description	Re-commits a stored PromptContext to a provider and streams the response as NDJSON. The original session is not modified; this is purely a debugging surface.
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

		messages := make([]provider.ChatMessage, 0, len(pc.MessageHashes))
		for _, mh := range pc.MessageHashes {
			obj, err := s.sessions.GetMessageObj(ctx, mh)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("resolve %s: %v", mh, err)})
				return
			}

			content := obj.Content
			if rendered, err := s.tmpl.RenderBody(obj.Content); err == nil {
				content = rendered
			}

			messages = append(messages, provider.ChatMessage{
				Role:    obj.Role,
				Content: content,
			})
		}

		stream, err := s.provider.Chat(ctx, providerName, modelName, messages, nil)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		out := make(chan PipelineEvent, 32)
		policy := s.config.Output.ResolveOutputPolicy()
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

			b, _ := json.Marshal(wire)

			c.Writer.Write(append(b, '\n'))
			flusher.Flush()
		}
	}
}
