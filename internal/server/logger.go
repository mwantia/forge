package server

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mwantia/forge/pkg/metrics"
)

func (s *Server) LoggerHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		s.logger.Debug("| %-5s %s | %-3d | %v", c.Request.Method, c.Request.URL.Path, status, duration)

		labels := []string{
			c.Request.Method,
			s.config.Server.Address,
			c.Request.URL.Path,
			strconv.Itoa(status),
		}

		metrics.ServerHttpRequestsTotal.WithLabelValues(labels...).Inc()
		metrics.ServerHttpRequestsDurationSeconds.WithLabelValues(labels...).Observe(duration.Seconds())
	}
}
