package server

import "github.com/gin-gonic/gin"

type HttpRouter interface {
	PublicGroup(relativePath string) *gin.RouterGroup
	AuthGroup(relativePath string) *gin.RouterGroup
}

func (s *ServerService) PublicGroup(relativePath string) *gin.RouterGroup {
	return s.public.Group(relativePath)
}

func (s *ServerService) AuthGroup(relativePath string) *gin.RouterGroup {
	return s.auth.Group(relativePath)
}
