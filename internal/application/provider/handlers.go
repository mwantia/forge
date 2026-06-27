package provider

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleListAllModels godoc
//
//	@Description	Returns a unified flat list of agents and raw provider models.
//	@Description	Query params: type=agent|model, provider=<name>, name=<substr>, sort=name|size|modified_at, order=asc|desc
func (s *ProviderService) handleListAllModels() gin.HandlerFunc {
	return func(c *gin.Context) {
		q := ListModelsQuery{
			Type:     c.Query("type"),
			Provider: c.Query("provider"),
			Name:     c.Query("name"),
			Sort:     c.Query("sort"),
			Order:    c.Query("order"),
		}
		models, err := s.ListAllModels(c.Request.Context(), q)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"models": models})
	}
}

// handleListProviders godoc
//
//	@Description	Returns names of all loaded provider plugins
func (s *ProviderService) handleListProviders() gin.HandlerFunc {
	return func(c *gin.Context) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		names := make([]string, 0, len(s.providers))
		for name := range s.providers {
			names = append(names, name)
		}
		c.JSON(http.StatusOK, gin.H{"providers": names})
	}
}

// handleGetProvider godoc
//
//	@Description	Returns info for a single provider plugin by name
func (s *ProviderService) handleGetProvider() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		p, err := s.getProvider(name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		lc := p.GetLifecycle()
		if lc == nil {
			c.JSON(http.StatusOK, gin.H{"name": name})
			return
		}
		c.JSON(http.StatusOK, gin.H{"name": name, "info": lc.GetPluginInfo()})
	}
}

// handleListModels godoc
//
//	@Description	Returns all models available from a provider
func (s *ProviderService) handleListModels() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")

		models, err := s.ListModels(c.Request.Context(), name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"models": models})
	}
}

// handleGetModel godoc
//
//	@Description	Returns info for a single model from a provider
func (s *ProviderService) handleGetModel() gin.HandlerFunc {
	return func(c *gin.Context) {
		providerName := c.Param("name")
		modelName := c.Param("model")

		model, err := s.GetModel(c.Request.Context(), providerName, modelName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, model)
	}
}

// handleEmbed godoc
//
//	@Description	Returns embedding vectors for the given content using the specified model.
//	@Description	model accepts "provider/model" or a forge alias (e.g. "forge/my-embed").
func (s *ProviderService) handleEmbed() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Model   string `json:"model" binding:"required"`
			Content string `json:"content" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()

		providerName, modelName, err := s.ResolveEmbedModel(ctx, req.Model)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		vecs, err := s.Embed(ctx, providerName, modelName, req.Content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"model": req.Model, "vectors": vecs})
	}
}
