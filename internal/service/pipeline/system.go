package pipeline

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/internal/service/session"
	"github.com/mwantia/forge/internal/service/session/dag"
)

// DefaultAgentSystem is the built-in agent-layer system prompt used when
// `pipeline { system = "..." }` is not set in the config. It is the most
// stable layer in the assembled prompt — placed first so cache prefixes
// stay valid across sessions and turns.
const DefaultAgentSystem = `You are a Forge agent — an LLM-driven assistant that operates through a curated set of tools provided by loaded plugins.

Reach for tools when they are clearly applicable. Prefer the most specific tool over the most general. When multiple tools could serve a request, pick the one whose description and guidance best match the user's intent. Read each tool's prose carefully — it documents when to use it and when not to.

Be concise. Surface tool results faithfully. Do not fabricate information you could verify with a tool call.`

// handleSystemShow godoc
//
//	@Summary		Get system message
//	@Description	Returns the system message (root of HEAD chain) for a session.
//	@Tags			sessions
//	@Produce		json
//	@Param			session_id	path		string	true	"Session ID"
//	@Success		200			{object}	map[string]any
//	@Failure		404			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/system [get]
func (s *PipelineService) handleSystemShow() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		meta, err := s.sessions.ResolveSession(ctx, c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		msgs, err := s.sessions.ListMessages(ctx, meta.ID, 0, 0)
		if err != nil || len(msgs) == 0 {
			c.JSON(http.StatusOK, gin.H{"hash": "", "content": "", "message": "no system message yet"})
			return
		}
		root := msgs[0] // Walk returns root-first; root is the system message.
		c.JSON(http.StatusOK, gin.H{"hash": root.Hash, "content": root.Content})
	}
}

type systemEditRequest struct {
	Content string `json:"content" binding:"required"`
}

// handleSystemEdit godoc
//
//	@Summary		Edit system message
//	@Description	Replaces the system message with template-rendered content. Creates a fork branch if the session already has messages; writes to HEAD directly on a fresh session.
//	@Tags			sessions
//	@Accept			json
//	@Produce		json
//	@Param			session_id	path		string				true	"Session ID"
//	@Param			body		body		systemEditRequest	true	"New content (template vars like ${session.id} are rendered)"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	map[string]string
//	@Failure		404			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/system [patch]
func (s *PipelineService) handleSystemEdit() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req systemEditRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx := c.Request.Context()
		meta, err := s.sessions.ResolveSession(ctx, c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		scoped, err := s.tmpl.Clone(session.SessionVars(meta))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "template clone failed: " + err.Error()})
			return
		}
		rendered, err := scoped.RenderBody(req.Content)
		if err != nil {
			s.logger.Warn("system edit: template render failed, using raw content", "session", meta.ID, "error", err)
			rendered = req.Content
		}

		hash, branch, err := s.writeSystemRoot(ctx, meta, rendered)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"hash": hash, "branch": branch})
	}
}

type systemRegenRequest struct {
	System         string   `json:"system"`
	ToolsVerbosity string   `json:"tools_verbosity"`
	Plugins        []string `json:"plugins"`
}

// handleSystemRegen godoc
//
//	@Summary		Regenerate system message
//	@Description	Re-assembles the system prompt from current plugin state and stores it as the new root MessageObj. On a fresh session (empty HEAD) it writes directly to HEAD; on an existing session it creates a fork branch.
//	@Tags			sessions
//	@Accept			json
//	@Produce		json
//	@Param			session_id	path		string				true	"Session ID"
//	@Param			body		body		systemRegenRequest	false	"Optional overrides"
//	@Success		200			{object}	map[string]any
//	@Failure		404			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/sessions/{session_id}/system/regen [post]
func (s *PipelineService) handleSystemRegen() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		meta, err := s.sessions.ResolveSession(ctx, c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		var req systemRegenRequest
		_ = c.ShouldBindJSON(&req) // body is optional

		effective := *meta
		if req.ToolsVerbosity != "" {
			effective.ToolsVerbosity = req.ToolsVerbosity
		}
		if len(req.Plugins) > 0 {
			effective.Plugins = req.Plugins
		}

		scoped, err := s.tmpl.Clone(session.SessionVars(&effective))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "template clone failed: " + err.Error()})
			return
		}
		modelSystem := s.fetchModelSystem(ctx, effective.Model)
		agentSystem := s.config.System
		if agentSystem == "" {
			agentSystem = DefaultAgentSystem
		}
		layers := collectPromptLayers(ctx, agentSystem, modelSystem, &effective, s.tools, s.logger)
		content := assembleSystemPrompt(layers, scoped, s.logger)

		// Append the optional session-layer override (template-rendered).
		if req.System != "" {
			extra, renderErr := scoped.RenderBody(req.System)
			if renderErr != nil {
				s.logger.Warn("system regen: session layer render failed, using raw", "session", meta.ID, "error", renderErr)
				extra = req.System
			}
			if extra != "" {
				if content != "" {
					content += "\n\n"
				}
				content += extra
			}
		}

		hash, branch, err := s.writeSystemRoot(ctx, meta, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"hash": hash, "branch": branch})
	}
}

// writeSystemRoot stores content as a root MessageObj (ParentHash="") and
// either advances HEAD directly (when it is empty) or creates a fork branch
// named "fork-<8hex>" (when HEAD already has messages). Returns the new hash
// and the branch name (empty when written to HEAD).
func (s *PipelineService) writeSystemRoot(ctx context.Context, meta *session.SessionMetadata, content string) (hash, branch string, err error) {
	headHash, err := s.sessions.HeadHash(ctx, meta.ID)
	if err != nil {
		return "", "", fmt.Errorf("read HEAD: %w", err)
	}

	sysMsg := &session.Message{Role: "system", Content: content}

	if headHash == "" {
		// Fresh session: write system as the first (root) message on HEAD.
		h, err := s.sessions.AppendMessageToRef(ctx, meta.ID, dag.HEAD, sysMsg)
		if err != nil {
			return "", "", fmt.Errorf("write system root: %w", err)
		}
		return h, "", nil
	}

	// Existing session: create a fork branch with just the new system as root.
	// The message has no parent (it's a new root), so we store it via PutMessageObj
	// and then create a ref pointing at it.
	sysObj := &dag.MessageObj{Role: "system", Content: content}
	h, err := s.sessions.PutMessageObj(ctx, sysObj)
	if err != nil {
		return "", "", fmt.Errorf("store system object: %w", err)
	}

	branchName := "fork-" + h[:8]
	for i := 2; ; i++ {
		existing, _ := s.sessions.ReadRef(ctx, meta.ID, branchName)
		if existing == "" {
			break
		}
		branchName = fmt.Sprintf("fork-%s-%d", h[:8], i)
	}
	if err := s.sessions.WriteRef(ctx, meta.ID, branchName, h); err != nil {
		return "", "", fmt.Errorf("create fork branch: %w", err)
	}
	return h, branchName, nil
}
