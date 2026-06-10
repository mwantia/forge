package ui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	apppipeline "github.com/mwantia/forge/internal/application/pipeline"
	uidag "github.com/mwantia/forge/internal/application/ui/templates/dag"
	tmplrefs "github.com/mwantia/forge/internal/application/ui/templates/refs"
	tmplsessions "github.com/mwantia/forge/internal/application/ui/templates/sessions"
	domsession "github.com/mwantia/forge/internal/domain/session"
)

type streamHandlers struct {
	sessions sessionReader
	renderer pipelineRenderer
	pipeline pipelineCommitter
}

var streamMD = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
	),
)

func mdToHTML(src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := streamMD.Convert([]byte(src), &buf); err != nil {
		return "<p>" + src + "</p>"
	}
	return buf.String()
}

func sseWrite(w io.Writer, flusher http.Flusher, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(data, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprintf(w, "\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func renderComponent(ctx context.Context, c templ.Component) string {
	var buf bytes.Buffer
	_ = c.Render(ctx, &buf)
	return buf.String()
}

func oobWrap(id, html string) string {
	return `<div id="` + id + `" hx-swap-oob="innerHTML">` + html + `</div>`
}

func (h *streamHandlers) handleStream() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		ref := c.Query("ref")

		job, ok := claimStreamJob(token)
		if !ok {
			c.String(http.StatusNotFound, "stream token not found or expired")
			return
		}

		ctx := c.Request.Context()

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)

		flusher, _ := c.Writer.(http.Flusher)
		if flusher != nil {
			flusher.Flush()
		}

		events, err := h.pipeline.CommitEvents(ctx, job.SessionID, job.Ref, job.Content)
		if err != nil {
			errHTML := `<div class="rounded border border-rem/30 bg-rem/5 px-3 py-2 text-rem text-ui-body">` + err.Error() + `</div>`
			sseWrite(c.Writer, flusher, "chunk", errHTML)
			return
		}

		var textBuf strings.Builder

		for ev := range events {
			switch e := ev.(type) {
			case apppipeline.ChunkEvent:
				if e.Text != "" {
					textBuf.WriteString(e.Text)
				}
				if e.Boundary == apppipeline.ChunkBoundaryBlock || e.Boundary == apppipeline.ChunkBoundaryFinal {
					if textBuf.Len() > 0 {
						sseWrite(c.Writer, flusher, "chunk", mdToHTML(textBuf.String()))
						textBuf.Reset()
					}
				}

			case apppipeline.ToolCallEvent:
				html := fmt.Sprintf(
					`<div class="rounded border border-line-soft bg-bg-1 font-mono flex items-center gap-2 px-2 py-1.5 mt-1"><span class="text-ui-label text-ink-4 uppercase tracking-wide shrink-0">call</span><span class="text-ui-meta text-ink-3 flex-1 truncate">%s</span></div>`,
					e.Name,
				)
				sseWrite(c.Writer, flusher, "chunk", html)

			case apppipeline.ToolResultEvent:
				errTag := ""
				if e.IsError {
					errTag = `<span class="text-rem text-ui-label shrink-0">err</span>`
				}
				html := fmt.Sprintf(
					`<div class="flex gap-3 items-start pl-10 mt-0.5"><div class="flex items-center gap-2 px-2 py-1.5 rounded border border-line-soft bg-bg-1 font-mono"><span class="text-ui-label text-ink-4 uppercase tracking-wide shrink-0">result</span><span class="text-ui-meta text-ink-3 flex-1 truncate">%s</span>%s</div></div>`,
					e.Name, errTag,
				)
				sseWrite(c.Writer, flusher, "chunk", html)

			case apppipeline.ErrorEvent:
				if textBuf.Len() > 0 {
					sseWrite(c.Writer, flusher, "chunk", mdToHTML(textBuf.String()))
				}
				errHTML := `<div class="rounded border border-rem/30 bg-rem/5 px-3 py-2 text-rem text-ui-body">` + e.Message + `</div>`
				sseWrite(c.Writer, flusher, "chunk", errHTML)
				return

			case apppipeline.DoneEvent:
				if textBuf.Len() > 0 {
					sseWrite(c.Writer, flusher, "chunk", mdToHTML(textBuf.String()))
				}
				sseWrite(c.Writer, flusher, "done", h.renderDoneOOB(ctx, job.SessionID, ref))
				return
			}
		}
	}
}

func (h *streamHandlers) renderDoneOOB(ctx context.Context, sessionID, ref string) string {
	var sb strings.Builder

	meta, err := h.sessions.ResolveSession(ctx, sessionID)
	if err != nil {
		return ""
	}

	refs, _ := h.sessions.ListRefs(ctx, sessionID)
	activeRef := resolveActiveRef(refs, ref)

	// Thread
	msgs, _ := h.sessions.ListMessagesFromRef(ctx, sessionID, activeRef, 0, 0)
	rendered := make([]*tmplsessions.RenderedMessage, len(msgs))
	for i, msg := range msgs {
		rm := &tmplsessions.RenderedMessage{Message: msg, Rendered: msg.Content}
		if h.renderer != nil {
			if r, err := h.renderer.RenderContent(ctx, sessionID, msg.Content); err == nil {
				rm.Rendered = r
			}
		}
		rendered[i] = rm
	}
	sb.WriteString(oobWrap("thread", renderComponent(ctx, tmplsessions.Thread(rendered, meta.ArchivedAt != nil))))

	// Refs panel
	sb.WriteString(oobWrap("refs-panel", renderComponent(ctx, tmplrefs.Panel(sessionID, refs))))

	// Node panel
	siblingCount := countSiblings(ctx, h.sessions, sessionID, activeRef, msgs, refs)
	sb.WriteString(oobWrap("node-panel", renderComponent(ctx, tmplsessions.NodePanel(sessionID, meta, msgs, activeRef, siblingCount))))

	// Mini DAG
	dagMsgs, _ := h.sessions.ListMessages(ctx, sessionID, 0, 200)
	nodes, edges := uidag.BuildMiniLayout(dagMsgs, refs)
	sb.WriteString(oobWrap("mini-dag", renderComponent(ctx, uidag.Mini(nodes, edges, sessionID))))

	return sb.String()
}

func countSiblings(ctx context.Context, sessions sessionReader, sessionID, activeRef string, messages []*domsession.Message, refs map[string]string) int {
	if len(messages) == 0 {
		return 0
	}
	headTip := messages[len(messages)-1]
	count := 0
	for name, refHash := range refs {
		if name == "HEAD" || name == activeRef || refHash == headTip.Hash {
			continue
		}
		obj, err := sessions.GetMessageObj(ctx, refHash)
		if err != nil {
			continue
		}
		if obj.ParentHash == headTip.ParentHash {
			count++
		}
	}
	return count
}
