package memory

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// handleStatus godoc
//
//	@Summary		Memory status
//	@Description	Returns whether a memory plugin is bound and which one.
//	@Tags			memory
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Security		BearerAuth
//	@Router			/v1/memory [get]
func (s *MemoryService) handleStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"backend": s.Backend(),
		})
	}
}

type storeResourceRequest struct {
	Content  string         `json:"content" binding:"required"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// handleStoreResource godoc
//
//	@Summary		Store a memory resource
//	@Description	Persists a resource into the given namespace.
//	@Tags			memory
//	@Accept			json
//	@Produce		json
//	@Param			namespace	path		string					true	"Namespace"
//	@Param			body		body		storeResourceRequest	true	"Resource to store"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	map[string]string
//	@Failure		503			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/memory/{namespace}/resources [post]
func (s *MemoryService) handleStoreResource() gin.HandlerFunc {
	return func(c *gin.Context) {
var req storeResourceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ns := c.Param("namespace")
		res, err := s.Store(c.Request.Context(), ns, req.Content, req.Metadata)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// handleRetrieveResources godoc
//
//	@Summary		Retrieve memory resources
//	@Description	Queries a namespace for resources matching the given query.
//	@Tags			memory
//	@Produce		json
//	@Param			namespace	path		string	true	"Namespace"
//	@Param			query		query		string	true	"Search query"
//	@Param			limit		query		int		false	"Max number of results (default 5)"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	map[string]string
//	@Failure		503			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/memory/{namespace}/resources [get]
func (s *MemoryService) handleRetrieveResources() gin.HandlerFunc {
	return func(c *gin.Context) {
query := c.Query("query")
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
			return
		}
		limit := 5
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		ns := c.Param("namespace")
		res, err := s.Retrieve(c.Request.Context(), ns, query, limit, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"resources": res})
	}
}
