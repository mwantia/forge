package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/registry"
)

type embedRequest struct {
	Model string `json:"model" binding:"required"`
	Input string `json:"input" binding:"required"`
}

type embedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

func Embed(reg *registry.PluginRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req embedRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		parts := strings.SplitN(req.Model, "/", 2)
		if len(parts) != 2 {
			respondError(c, http.StatusBadRequest, "bad_request", "model must be in 'provider/model' format")
			return
		}
		ctx := c.Request.Context()
		p, err := reg.GetProvider(ctx, parts[0])
		if err != nil {
			respondError(c, http.StatusNotFound, "not_found", fmt.Sprintf("provider '%s' not found", parts[0]))
			return
		}
		vecs, err := p.Embed(ctx, req.Input, &plugins.Model{ModelName: parts[1]})
		if err != nil {
			respondError(c, http.StatusBadGateway, "provider_error", err.Error())
			return
		}
		c.JSON(http.StatusOK, embedResponse{
			Model:      req.Model,
			Embeddings: vecs,
		})
	}
}
