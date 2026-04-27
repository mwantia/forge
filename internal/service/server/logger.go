package server

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *ServerService) loggerHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		s.logger.Debug(fmt.Sprintf("| %-5s %s | %-3d | %v", c.Request.Method, c.Request.URL.Path, status, duration))

		// Use the matched route template (e.g. /plugins/:name) as the path
		// label. c.FullPath() returns "" for unmatched routes — skip those to
		// avoid flooding metrics with arbitrary attacker/scanner paths.
		path := c.FullPath()
		if path == "" {
			return
		}

		method := c.Request.Method
		statusLabel := strconv.Itoa(status)

		ServerHttpRequestsTotal.WithLabelValues(method, path, statusLabel).Inc()
		ServerHttpRequestsDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	}
}
