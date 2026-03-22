package log

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type LoggerWrapper struct {
	out   io.Writer
	color bool
}

func LogWrapper(out io.Writer, color bool) *LoggerWrapper {
	return &LoggerWrapper{
		out:   out,
		color: color,
	}
}

func (w *LoggerWrapper) Write(p []byte) (int, error) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	str := string(p)
	clr := w.GetLogLevelColor(str)

	line := fmt.Sprintf("%s[%s] %v", clr, ts, str)
	return w.out.Write([]byte(line))
}

func (w *LoggerWrapper) GetLogLevelColor(s string) string {
	if w.color {
		if strings.HasPrefix(s, "[TRACE]") {
			return "\033[36m" // Cyan for trace
		}
		if strings.HasPrefix(s, "[DEBUG]") {
			return "\033[34m" // Blue for debug
		}
		if strings.HasPrefix(s, "[INFO]") {
			return "\033[32m" // Green for info
		}
		if strings.HasPrefix(s, "[WARN]") {
			return "\033[33m" // Yellow for warn
		}
		if strings.HasPrefix(s, "[ERROR]") {
			return "\033[31m" // Red for error
		}
	}
	return "\033[0m"
}
