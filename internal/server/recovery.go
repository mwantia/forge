package server

import "github.com/gin-gonic/gin"

func (impl *Server) Recovery() gin.HandlerFunc {
	return gin.Recovery()
}
