package server

import "github.com/gin-gonic/gin"

type HttpRouter interface {
	PublicGroup(relativePath string) *gin.RouterGroup
	AuthGroup(relativePath string) *gin.RouterGroup
	// UIGroup mounts at the engine root (not under /v1/) with auth middleware.
	// Use this for browser-facing routes that should not share the /v1/ API prefix.
	UIGroup(relativePath string) *gin.RouterGroup
}

func (s *ServerService) PublicGroup(relativePath string) *gin.RouterGroup {
	return s.public.Group(relativePath)
}

func (s *ServerService) AuthGroup(relativePath string) *gin.RouterGroup {
	return s.auth.Group(relativePath)
}

func (s *ServerService) UIGroup(relativePath string) *gin.RouterGroup {
	return s.engine.Group(relativePath, s.authMiddleware(), noCacheMiddleware())
}

// noCacheMiddleware prevents browsers and proxies from caching HTML/HTMX responses.
// Static assets under /ui/assets/ are served with their own immutable headers
// and bypass this middleware because UIService registers them after this group.
func noCacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}
