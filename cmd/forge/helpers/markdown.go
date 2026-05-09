package helpers

import "github.com/charmbracelet/glamour"

func RenderMarkdown(content string, noRender bool) string {
	if noRender {
		return content
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return content
	}
	out, err := r.Render(content)
	if err != nil {
		return content
	}
	return out
}
