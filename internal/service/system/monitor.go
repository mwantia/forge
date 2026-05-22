package system

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	forgelog "github.com/mwantia/forge/internal/log"
)

type logSink struct {
	level  hclog.Level
	ch     chan string
	closed atomic.Bool
}

func (s *logSink) Accept(name string, level hclog.Level, msg string, args ...interface{}) {
	if level < s.level || s.closed.Load() {
		return
	}
	line := formatEntry(name, level, msg, args)
	select {
	case s.ch <- line:
	default:
	}
}

func formatEntry(name string, level hclog.Level, msg string, args []interface{}) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "[%s]", strings.ToUpper(level.String()))
	if name != "" {
		sb.WriteString(" ")
		sb.WriteString(name)
		sb.WriteString(":")
	}
	sb.WriteString(" ")
	sb.WriteString(msg)
	for i := 0; i+1 < len(args); i += 2 {
		fmt.Fprintf(&sb, "  %v=%v", args[i], args[i+1])
	}
	return sb.String()
}

// handleMonitor godoc
//
//	@Description	Opens a long-lived connection and streams formatted log lines, one per chunk. ?level= filters (trace/debug/info/warn/error). Disconnect to stop.
func (s *SystemService) handleMonitor() gin.HandlerFunc {
	return func(c *gin.Context) {
		levelStr := c.DefaultQuery("level", "info")
		level := hclog.LevelFromString(levelStr)
		if level == hclog.NoLevel {
			level = hclog.Info
		}

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
			return
		}

		sink := &logSink{
			level: level,
			ch:    make(chan string, 512),
		}
		forgelog.RegisterSink(sink)
		defer func() {
			sink.closed.Store(true)
			forgelog.DeregisterSink(sink)
		}()

		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)

		ctx := c.Request.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-sink.ch:
				if !ok {
					return
				}
				fmt.Fprintf(c.Writer, "%s\n", line)
				flusher.Flush()
			}
		}
	}
}
