package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	appsession "github.com/mwantia/forge/internal/application/session"
	"github.com/mwantia/forge/internal/infrastructure/storage/dag"
)

type systemResetRequest struct {
	System         string   `json:"system"`
	ToolsVerbosity string   `json:"tools_verbosity"`
	Plugins        []string `json:"plugins"`
}

type systemResetResult struct {
	Hash   string `json:"hash"`
	Branch string `json:"branch,omitempty"`
}

// handleResetSystemSnapshot godoc
//
//	@Description	Re-assembles the system prompt from current plugin state and stores it as the root message. Creates a fork branch when HEAD is non-empty.
func (s *PipelineService) handleResetSystemSnapshot() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var req systemResetRequest
		_ = c.ShouldBindJSON(&req) // all fields optional

		meta, err := s.sessions.ResolveSession(ctx, c.Param("session_id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		// Apply request overrides onto a copy so prompt assembly uses
		// caller-supplied values without mutating the stored session.
		effective := *meta
		if req.ToolsVerbosity != "" {
			effective.ToolsVerbosity = req.ToolsVerbosity
		}
		if len(req.Plugins) > 0 {
			effective.Plugins = req.Plugins
		}

		scoped, err := s.tmpl.Clone(appsession.SessionVars(meta))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		agentSystem := s.config.System
		if agentSystem == "" {
			agentSystem = DefaultAgentSystem
		}
		layers := collectPromptLayers(ctx, agentSystem, s.fetchModelSystem(ctx, effective.Model), &effective, s.tools, s.logger)
		content := assembleSystemPrompt(layers, scoped, s.logger)

		// Optional session-layer fragment appended after the assembled prompt.
		if req.System != "" {
			rendered, err := scoped.RenderBody(req.System)
			if err == nil {
				req.System = strings.TrimSpace(rendered)
			}
			if req.System != "" {
				content = strings.TrimSpace(content) + "\n\n" + req.System
			}
		}

		ref, branch, err := s.freshSystemRef(ctx, meta.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		sysMsg := &appsession.Message{Role: "system", Content: content}
		hash, err := s.sessions.AppendMessageToRef(ctx, meta.ID, ref, sysMsg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, systemResetResult{Hash: hash, Branch: branch})
	}
}

// freshSystemRef returns the ref to write a new system message to.
// If HEAD is empty returns dag.HEAD with an empty branch name.
// If HEAD already has messages, creates a new empty fork-<8hex> branch.
func (s *PipelineService) freshSystemRef(ctx context.Context, sessionID string) (ref, branch string, err error) {
	headHash, err := s.sessions.ReadRef(ctx, sessionID, dag.HEAD)
	if err != nil {
		return "", "", fmt.Errorf("read HEAD: %w", err)
	}
	if headHash == "" {
		return dag.HEAD, "", nil
	}

	var buf [4]byte
	if _, err = rand.Read(buf[:]); err != nil {
		return "", "", fmt.Errorf("rand: %w", err)
	}
	name := "fork-" + hex.EncodeToString(buf[:])
	if err := s.sessions.WriteRef(ctx, sessionID, name, ""); err != nil {
		return "", "", fmt.Errorf("create fork ref: %w", err)
	}
	return name, name, nil
}
