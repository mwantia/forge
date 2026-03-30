package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/session"
)

func ListSessions(mgr *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		opts := session.ListOptions{Limit: limit, Offset: offset}
		if p := c.Query("parent"); p != "" || c.Request.URL.Query().Has("parent") {
			opts.ParentID = &p
		}

		sessions, err := mgr.List(opts)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"sessions": sessions})
	}
}

func CreateSession(mgr *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var opts session.CreateOptions
		if err := c.ShouldBindJSON(&opts); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		sess, err := mgr.Create(opts)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.JSON(http.StatusCreated, sess)
	}
}

func GetSession(mgr *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, err := mgr.Get(c.Param("id"))
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", err.Error())
			return
		}
		c.JSON(http.StatusOK, sess)
	}
}

func DeleteSession(mgr *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if _, err := mgr.Get(id); err != nil {
			respondError(c, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err := mgr.Delete(id); err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	}
}

type addMessageRequest struct {
	Content string `json:"content" binding:"required"`
	Stream  bool   `json:"stream"`
}

func AddMessage(mgr *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req addMessageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		stream, err := mgr.Dispatch(c.Request.Context(), id, req.Content)
		if err != nil {
			respondError(c, http.StatusBadRequest, "dispatch_error", err.Error())
			return
		}

		if req.Stream {
			streamChat(c, stream)
			return
		}

		result, err := plugins.CollectStream(stream)
		if err != nil {
			respondError(c, http.StatusBadGateway, "provider_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func ListSessionTools(mgr *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		tools, err := mgr.ListTools(c.Request.Context(), id)
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"tools": tools})
	}
}

func GetMessage(mgr *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		msg, err := mgr.GetMessage(c.Param("id"), c.Param("message_id"))
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", err.Error())
			return
		}
		c.JSON(http.StatusOK, msg)
	}
}

func CompactMessages(mgr *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var opts session.CompactOptions
		if err := c.ShouldBindJSON(&opts); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if !opts.StripTools {
			respondError(c, http.StatusBadRequest, "bad_request", "no compaction options specified")
			return
		}
		result, err := mgr.Compact(id, opts)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func ListMessages(mgr *session.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		messages, err := mgr.GetMessages(id, limit, offset)
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"messages": messages})
	}
}
