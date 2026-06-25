package approvals

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge-sdk/pkg/plugin/base"
	domapprovals "github.com/mwantia/forge/internal/domain/approvals"
)

func (s *ApprovalService) handleList() gin.HandlerFunc {
	return func(c *gin.Context) {
		statusParam := c.DefaultQuery("status", "all")
		pluginParam := c.Query("plugin")

		filter := domapprovals.ApprovalFilter{Plugin: pluginParam}

		switch statusParam {
		case "pending":
			filter.Status = []domapprovals.ApprovalStatus{domapprovals.StatusPending}

		case "denied":
			filter.Status = []domapprovals.ApprovalStatus{domapprovals.StatusDenied}

		case "resolved":
			filter.Status = []domapprovals.ApprovalStatus{
				domapprovals.StatusAllowed,
				domapprovals.StatusDenied,
				domapprovals.StatusTimeout,
			}

		default:
			// all — no status filter
		}

		records, err := s.List(filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"approvals": records})
	}
}

func (s *ApprovalService) handleStream() gin.HandlerFunc {
	return func(c *gin.Context) {
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)

		ctx := c.Request.Context()
		ch := s.Subscribe(ctx)

		for {
			select {
			case <-ctx.Done():
				return

			case rec, ok := <-ch:
				if !ok {
					return
				}

				data, err := json.Marshal(rec)
				if err != nil {
					continue
				}

				fmt.Fprintf(c.Writer, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

func (s *ApprovalService) handleGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		rec, err := s.Get(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"approval": rec})
	}
}

type respondRequest struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

func (s *ApprovalService) handleRespond() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req respondRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ad := base.ApprovalDecision{
			Allow:  req.Allow,
			Reason: req.Reason,
		}
		if err := s.Respond(id, ad); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func (s *ApprovalService) handleCancel() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		if err := s.Cancel(id); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func matchGlob(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	return err == nil && matched
}
