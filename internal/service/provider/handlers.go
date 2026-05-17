package provider

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// handleListAllModels godoc
//
//	@Summary		List all models
//	@Description	Returns all models from every provider alongside locally configured model templates
//	@Tags			provider
//	@Produce		json
//	@Param			type	query		string	false	"Filter local models by type (chat|embed)"
//	@Success		200	{object}	object
//	@Failure		500	{object}	map[string]string
//	@Router			/v1/provider/models [get]
func (s *ProviderService) handleListAllModels() gin.HandlerFunc {
	return func(c *gin.Context) {
		if kind := c.Query("type"); kind != "" {
			local, err := s.ListModelsByType(c.Request.Context(), kind)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"local": local})
			return
		}
		models, local, err := s.ListAllModels(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"models": models, "local": local})
	}
}

// handleListProviders godoc
//
//	@Summary		List providers
//	@Description	Returns names of all loaded provider plugins
//	@Tags			provider
//	@Produce		json
//	@Success		200	{object}	map[string][]string
//	@Router			/v1/provider/ [get]
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
//	@Summary		Get provider
//	@Description	Returns info for a single provider plugin by name
//	@Tags			provider
//	@Produce		json
//	@Param			name	path		string	true	"Provider name"
//	@Success		200		{object}	object
//	@Failure		404		{object}	map[string]string
//	@Router			/v1/provider/{name} [get]
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
//	@Summary		List models
//	@Description	Returns all models available from a provider
//	@Tags			provider
//	@Produce		json
//	@Param			name	path		string	true	"Provider name"
//	@Success		200		{object}	object
//	@Failure		500		{object}	map[string]string
//	@Router			/v1/provider/{name}/models [get]
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
//	@Summary		Get model
//	@Description	Returns info for a single model from a provider
//	@Tags			provider
//	@Produce		json
//	@Param			name	path		string	true	"Provider name"
//	@Param			model	path		string	true	"Model name"
//	@Success		200		{object}	object
//	@Failure		404		{object}	map[string]string
//	@Router			/v1/provider/{name}/models/{model} [get]
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
//	@Summary		Embed text
//	@Description	Returns embedding vectors for the given content using the specified model.
//	@Description	model accepts "provider/model" or a forge alias (e.g. "forge/my-embed").
//	@Tags			provider
//	@Accept			json
//	@Produce		json
//	@Param			body	body		map[string]any	true	"{ model: string, content: string }"
//	@Success		200		{object}	map[string]any
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		BearerAuth
//	@Router			/v1/provider/embed [post]
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
