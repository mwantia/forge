package log

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type ColouredWriter struct {
	writer io.Writer
}

func NewColouredWriter(writer io.Writer) *ColouredWriter {
	return &ColouredWriter{
		writer: writer,
	}
}

func (w *ColouredWriter) Write(p []byte) (int, error) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	str := string(p)
	clr := w.identifyLogLevel(str)

	line := fmt.Sprintf("%s[%s] %v", clr, ts, str)
	_, err := w.writer.Write([]byte(line))
	return len(p), err
}

func (w *ColouredWriter) identifyLogLevel(s string) string {
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

	return ""
}
