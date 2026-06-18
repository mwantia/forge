package ui

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	apppipeline "github.com/mwantia/forge/internal/application/pipeline"
	appsession "github.com/mwantia/forge/internal/application/session"
	uidag "github.com/mwantia/forge/internal/application/ui/templates/dag"
	tmplrefs "github.com/mwantia/forge/internal/application/ui/templates/refs"
	tmplsessions "github.com/mwantia/forge/internal/application/ui/templates/sessions"
)

type streamHandlers struct {
	sessions  sessionReader
	tools     namespaceLister
	renderer  pipelineRenderer
	pipeline  pipelineCommitter
	providers modelLister
}

func sseWrite(w gin.ResponseWriter, flusher http.Flusher, event, data string) {
	w.WriteString("event: " + event + "\n")
	for _, line := range strings.Split(data, "\n") {
		w.WriteString("data: " + line + "\n")
	}
	w.WriteString("\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func writeComponent(ctx context.Context, sb *strings.Builder, c templ.Component) {
	var buf bytes.Buffer
	_ = c.Render(ctx, &buf)
	sb.WriteString(buf.String())
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
			var sb strings.Builder
			writeComponent(ctx, &sb, tmplsessions.StreamErrorBlock(err.Error()))
			sseWrite(c.Writer, flusher, "chunk", sb.String())
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
						sseWrite(c.Writer, flusher, "chunk", renderMarkdown(textBuf.String()))
						textBuf.Reset()
					}
				}

			case apppipeline.ToolCallEvent:
				var sb strings.Builder
				writeComponent(ctx, &sb, tmplsessions.StreamToolCallChip(e.Name))
				sseWrite(c.Writer, flusher, "chunk", sb.String())

			case apppipeline.ToolResultEvent:
				var sb strings.Builder
				writeComponent(ctx, &sb, tmplsessions.StreamToolResultChip(e.Name, e.IsError))
				sseWrite(c.Writer, flusher, "chunk", sb.String())

			case apppipeline.ErrorEvent:
				if textBuf.Len() > 0 {
					sseWrite(c.Writer, flusher, "chunk", renderMarkdown(textBuf.String()))
				}
				var sb strings.Builder
				writeComponent(ctx, &sb, tmplsessions.StreamErrorBlock(e.Message))
				sseWrite(c.Writer, flusher, "chunk", sb.String())
				return

			case apppipeline.DoneEvent:
				if textBuf.Len() > 0 {
					sseWrite(c.Writer, flusher, "chunk", renderMarkdown(textBuf.String()))
				}
				sseWrite(c.Writer, flusher, "done", h.renderDoneOOB(ctx, job.SessionID, ref))
				return
			}
		}
	}
}

func (h *streamHandlers) renderDoneOOB(ctx context.Context, sessionID, ref string) string {
	meta, err := h.sessions.ResolveSession(ctx, sessionID)
	if err != nil {
		return ""
	}

	refs, _ := h.sessions.ListRefs(ctx, sessionID)
	activeRef := resolveActiveRef(refs, ref)

	msgs, _ := h.sessions.ListMessagesFromRef(ctx, sessionID, activeRef, 0, 0)
	rendered := renderMessages(ctx, h.renderer, sessionID, msgs)

	allPlugins := pluginNamespacesFrom(h.tools)
	subSessions, _ := h.sessions.QuerySessions(ctx, appsession.SessionQuery{ParentID: sessionID})

	dagMsgs, _ := h.sessions.ListMessages(ctx, sessionID, 0, 200)
	nodes, edges := uidag.BuildMiniLayout(dagMsgs, refs)

	var sb strings.Builder
	for _, comp := range []templ.Component{
		tmplsessions.ThreadOOB(rendered, meta.ArchivedAt != nil),
		tmplrefs.RefsPanelOOB(sessionID, refs),
		tmplsessions.SessionInfoCardOOB(sessionID, meta, allPlugins, meta.ArchivedAt == nil, lastAssistantTokens(msgs), resolveWindowSize(ctx, h.providers, meta)),
		tmplsessions.SiblingsSectionOOB(subSessions),
		tmplsessions.PathSectionOOB(msgs),
		uidag.MiniOOB(nodes, edges, sessionID),
	} {
		writeComponent(ctx, &sb, comp)
	}
	return sb.String()
}
