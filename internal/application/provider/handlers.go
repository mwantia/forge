package provider

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// handleListAllModels godoc
//
//	@Description	Returns a unified flat list of all models: forge aliases (without the forge/ prefix) followed by provider-supplied models.
//	@Description	Pass ?type=chat or ?type=embed to filter to locally configured aliases of that type.
func (s *ProviderService) handleListAllModels() gin.HandlerFunc {
	return func(c *gin.Context) {
		models, err := s.ListAllModels(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if kind := c.Query("type"); kind != "" {
			filtered := models[:0:0]
			for _, m := range models {
				if m.Type == kind {
					filtered = append(filtered, m)
				}
			}
			c.JSON(http.StatusOK, gin.H{"models": filtered})
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

		// Resolve forge alias or split "provider/model".
		var providerName, modelName string
		if strings.HasPrefix(req.Model, "forge/") {
			p, m, err := s.ResolveEmbedModel(ctx, strings.TrimPrefix(req.Model, "forge/"))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			providerName, modelName = p, m
		} else if p, m, ok := strings.Cut(req.Model, "/"); ok {
			providerName, modelName = p, m
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "model must be \"provider/model\" or \"forge/<alias>\""})
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
