package resource

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// handleStatus godoc
//
//	@Summary		Resource status
//	@Description	Returns whether a resource plugin is bound and which one.
//	@Tags			resource
//	@Produce		json
//	@Success		200	{object}	map[string]any
//	@Security		BearerAuth
//	@Router			/v1/resources [get]
func (s *ResourceService) handleStatus() gin.HandlerFunc {
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
//	@Summary		Store a resource
//	@Description	Persists a resource into the given namespace.
//	@Tags			resource
//	@Accept			json
//	@Produce		json
//	@Param			namespace	path		string					true	"Namespace"
//	@Param			body		body		storeResourceRequest	true	"Resource to store"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	map[string]string
//	@Failure		503			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/resources/{namespace} [post]
func (s *ResourceService) handleStoreResource() gin.HandlerFunc {
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

// handleListResources godoc
//
//	@Summary		List resources
//	@Description	Returns all resources in a namespace.
//	@Tags			resource
//	@Produce		json
//	@Param			namespace	path		string	true	"Namespace"
//	@Success		200			{object}	map[string]any
//	@Failure		500			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/resources/{namespace} [get]
func (s *ResourceService) handleListResources() gin.HandlerFunc {
	return func(c *gin.Context) {
		res, err := s.List(c.Request.Context(), c.Param("namespace"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"resources": res})
	}
}

// handleRecallResources godoc
//
//	@Summary		Recall resources
//	@Description	Semantic-search a namespace for resources matching q.
//	@Tags			resource
//	@Produce		json
//	@Param			namespace	path		string	true	"Namespace"
//	@Param			q			query		string	true	"Search query"
//	@Param			limit		query		int		false	"Max number of results (default 5)"
//	@Success		200			{object}	map[string]any
//	@Failure		400			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/resources/{namespace}/recall [get]
func (s *ResourceService) handleRecallResources() gin.HandlerFunc {
	return func(c *gin.Context) {
		ns := c.Param("namespace")
		query := c.Query("q")
		if query == "" {
			query = c.Query("query")
		}
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing query parameter q"})
			return
		}
		limit := 5
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		res, err := s.Recall(c.Request.Context(), ns, query, limit, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"resources": res})
	}
}

// handleGetResource godoc
//
//	@Summary		Get a resource by id
//	@Description	Returns the resource at namespace/id.
//	@Tags			resource
//	@Produce		json
//	@Param			namespace	path		string	true	"Namespace"
//	@Param			id			path		string	true	"Resource ID"
//	@Success		200			{object}	map[string]any
//	@Failure		404			{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/resources/{namespace}/{id} [get]
func (s *ResourceService) handleGetResource() gin.HandlerFunc {
	return func(c *gin.Context) {
		res, err := s.Get(c.Request.Context(), c.Param("namespace"), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// handleForgetResource godoc
//
//	@Summary		Forget a resource
//	@Description	Removes the resource at namespace/id. Missing IDs are not an error.
//	@Tags			resource
//	@Param			namespace	path	string	true	"Namespace"
//	@Param			id			path	string	true	"Resource ID"
//	@Success		204
//	@Failure		500	{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/resources/{namespace}/{id} [delete]
func (s *ResourceService) handleForgetResource() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := s.Forget(c.Request.Context(), c.Param("namespace"), c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
