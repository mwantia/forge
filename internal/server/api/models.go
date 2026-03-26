package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/pkg/plugins"
)

type modelResponse struct {
	Provider string `json:"provider"`
	*plugins.Model
}

type listModelsResponse struct {
	Models []modelResponse `json:"models"`
}

func ListModels(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		providers := reg.GetAllProviders(ctx)
		resp := listModelsResponse{Models: make([]modelResponse, 0)}
		for name, p := range providers {
			models, err := p.ListModels(ctx)
			if err != nil {
				continue
			}
			for _, m := range models {
				resp.Models = append(resp.Models, modelResponse{Provider: name, Model: m})
			}
		}
		c.JSON(http.StatusOK, resp)
	}
}

func ListProviderModels(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		providerName := c.Param("provider")
		p, err := reg.GetProvider(ctx, providerName)
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", "provider not found")
			return
		}
		models, err := p.ListModels(ctx)
		if err != nil {
			respondError(c, http.StatusBadGateway, "provider_error", err.Error())
			return
		}
		resp := listModelsResponse{Models: make([]modelResponse, 0, len(models))}
		for _, m := range models {
			resp.Models = append(resp.Models, modelResponse{Provider: providerName, Model: m})
		}
		c.JSON(http.StatusOK, resp)
	}
}

type createModelRequest struct {
	ModelName string               `json:"model_name" binding:"required"`
	Template  *plugins.ModelTemplate `json:"template"`
}

func CreateModel(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		providerName := c.Param("provider")
		var req createModelRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		p, err := reg.GetProvider(ctx, providerName)
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", "provider not found")
			return
		}
		model, err := p.CreateModel(ctx, req.ModelName, req.Template)
		if err != nil {
			respondError(c, http.StatusBadGateway, "provider_error", err.Error())
			return
		}
		c.JSON(http.StatusCreated, modelResponse{Provider: providerName, Model: model})
	}
}

func DeleteModel(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		providerName := c.Param("provider")
		modelName := c.Param("model")
		p, err := reg.GetProvider(ctx, providerName)
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", "provider not found")
			return
		}
		ok, err := p.DeleteModel(ctx, modelName)
		if err != nil {
			respondError(c, http.StatusBadGateway, "provider_error", err.Error())
			return
		}
		if !ok {
			respondError(c, http.StatusNotFound, "not_found", "model not found")
			return
		}
		c.Status(http.StatusNoContent)
	}
}
