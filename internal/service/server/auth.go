package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *ServerService) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.config.Token == "" {
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || token != s.config.Token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "unauthorized",
					"message": "invalid or missing bearer token",
				},
			})
			return
		}
		c.Next()
	}
}
