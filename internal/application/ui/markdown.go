package ui

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
	),
)

func renderMarkdown(src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(src), &buf); err != nil {
		return "<p>" + src + "</p>"
	}
	return buf.String()
}
