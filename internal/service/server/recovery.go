package server

import "github.com/gin-gonic/gin"

func (impl *ServerService) recovery() gin.HandlerFunc {
	return gin.Recovery()
}
