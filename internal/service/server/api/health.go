package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health godoc
//
//	@Description	Returns OK when the server is running
func Health() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"Status": "OK",
		})
	}
}
