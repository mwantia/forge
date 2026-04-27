package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health godoc
//
//	@Summary		Health check
//	@Description	Returns OK when the server is running
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Router			/v1/health [get]
func Health() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"Status": "OK",
		})
	}
}
