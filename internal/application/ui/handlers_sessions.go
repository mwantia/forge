package ui

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	appsession "github.com/mwantia/forge/internal/application/session"
	tmplsessions "github.com/mwantia/forge/internal/application/ui/templates/sessions"
	domprovider "github.com/mwantia/forge/internal/domain/provider"
)

type sessionHandlers struct {
	sessions  sessionReader
	tools     namespaceLister
	renderer  pipelineRenderer
	providers modelLister
}

func (h *sessionHandlers) handleList() gin.HandlerFunc {
	return func(c *gin.Context) {
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

		q := appsession.SessionQuery{
			Search: c.Query("search"),
			Offset: offset,
			Limit:  limit,
		}
		if a := c.Query("archived"); a != "" {
			v := a == "true"
			q.Archived = &v
		}
		if p := c.Query("plugins"); p != "" {
			for _, part := range strings.Split(p, ",") {
				if t := strings.TrimSpace(part); t != "" {
					q.Plugins = append(q.Plugins, t)
				}
			}
		}

		sessions, err := h.sessions.QuerySessions(c.Request.Context(), q)
		if err != nil {
			c.String(http.StatusInternalServerError, "failed to list sessions: %v", err)
			return
		}

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplsessions.List(sessions, h.pluginNamespaces()).Render(c.Request.Context(), c.Writer)
	}
}

func (h *sessionHandlers) handleNew() gin.HandlerFunc {
	return func(c *gin.Context) {
		var models []*domprovider.ProviderModelTemplate
		if h.providers != nil {
			models, _ = h.providers.ListModelsByType(c.Request.Context(), domprovider.ModelTypeChat)
		}

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplsessions.New(models, h.pluginNamespaces()).Render(c.Request.Context(), c.Writer)
	}
}

func (h *sessionHandlers) handleCreate() gin.HandlerFunc {
	return func(c *gin.Context) {
		model := c.PostForm("model")
		name := c.PostForm("name")
		title := c.PostForm("title")

		if model == "" {
			c.String(http.StatusBadRequest, "model is required")
			return
		}

		var pluginConfigs []appsession.PluginConfig
		for _, pname := range c.PostFormArray("plugins") {
			if pname = strings.TrimSpace(pname); pname != "" {
				pluginConfigs = append(pluginConfigs, appsession.PluginConfig{Name: pname, Enabled: true})
			}
		}

		meta, err := h.sessions.CreateSession(c.Request.Context(), model, name, title, "", "", pluginConfigs)
		if err != nil {
			c.String(http.StatusConflict, "create failed: %v", err)
			return
		}

		c.Header("HX-Redirect", "/ui/sessions/"+meta.ID)
		c.Status(http.StatusSeeOther)
	}
}

func (h *sessionHandlers) handleDetail() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")

		meta, err := h.sessions.ResolveSession(ctx, id)
		if err != nil {
			c.String(http.StatusNotFound, "not found: %v", err)
			return
		}

		ref := c.DefaultQuery("ref", "HEAD")
		raw, _ := h.sessions.ListMessagesFromRef(ctx, meta.ID, ref, 0, 0)
		refs, _ := h.sessions.ListRefs(ctx, meta.ID)
		activeRef := resolveActiveRef(refs, ref)

		messages := renderMessages(ctx, h.renderer, meta.ID, raw)

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplsessions.Detail(meta, messages, refs, activeRef).Render(ctx, c.Writer)
	}
}

func (h *sessionHandlers) handleDelete() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.sessions.DeleteSession(c.Request.Context(), c.Param("id")); err != nil {
			c.String(http.StatusInternalServerError, "delete failed: %v", err)
			return
		}
		c.Header("HX-Redirect", "/ui/sessions")
		c.Status(http.StatusOK)
	}
}

func (h *sessionHandlers) handleThread() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")

		meta, err := h.sessions.ResolveSession(ctx, id)
		if err != nil {
			c.String(http.StatusNotFound, "not found: %v", err)
			return
		}

		refs, _ := h.sessions.ListRefs(ctx, meta.ID)
		activeRef := resolveActiveRef(refs, c.DefaultQuery("ref", ""))
		raw, _ := h.sessions.ListMessagesFromRef(ctx, meta.ID, activeRef, 0, 0)

		messages := renderMessages(ctx, h.renderer, meta.ID, raw)

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplsessions.Thread(messages, meta.ArchivedAt != nil).Render(ctx, c.Writer)
	}
}

func (h *sessionHandlers) handleNodePanel() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")

		meta, err := h.sessions.ResolveSession(ctx, id)
		if err != nil {
			c.String(http.StatusNotFound, "not found: %v", err)
			return
		}

		refs, _ := h.sessions.ListRefs(ctx, id)
		ref := c.DefaultQuery("ref", "HEAD")
		activeRef := resolveActiveRef(refs, ref)

		messages, _ := h.sessions.ListMessagesFromRef(ctx, id, activeRef, 0, 0)
		subSessions, _ := h.sessions.QuerySessions(ctx, appsession.SessionQuery{ParentID: id})

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplsessions.LeftPanelResponse(id, meta, messages, activeRef, subSessions, h.pluginNamespaces(), lastAssistantTokens(messages), resolveWindowSize(ctx, h.providers, meta)).Render(ctx, c.Writer)
	}
}

func (h *sessionHandlers) handleEdit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")

		meta, err := h.sessions.ResolveSession(ctx, id)
		if err != nil {
			c.String(http.StatusNotFound, "not found: %v", err)
			return
		}
		if meta.ArchivedAt != nil {
			c.String(http.StatusConflict, "session is archived")
			return
		}

		var models []*domprovider.ProviderModelTemplate
		if h.providers != nil {
			models, _ = h.providers.ListModelsByType(ctx, domprovider.ModelTypeChat)
		}

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplsessions.SessionEditForm(meta.ID, meta, models).Render(ctx, c.Writer)
	}
}

func (h *sessionHandlers) handleUpdate() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")

		meta, err := h.sessions.ResolveSession(ctx, id)
		if err != nil {
			c.String(http.StatusNotFound, "not found: %v", err)
			return
		}
		if meta.ArchivedAt != nil {
			c.String(http.StatusConflict, "session is archived")
			return
		}

		if name := strings.TrimSpace(c.PostForm("name")); name != "" && name != meta.Name {
			// check uniqueness: if another session resolves to this name, reject
			if existing, err := h.sessions.ResolveSession(ctx, name); err == nil && existing.ID != meta.ID {
				c.String(http.StatusConflict, "name already in use: %s", name)
				return
			}
			meta.Name = name
		}
		if title := c.PostForm("title"); title != meta.Title {
			meta.Title = title
		}
		if desc := c.PostForm("description"); desc != meta.Description {
			meta.Description = desc
		}
		if model := strings.TrimSpace(c.PostForm("model")); model != "" && model != meta.Model {
			meta.Model = model
		}

		if err := h.sessions.SaveSession(ctx, meta); err != nil {
			c.String(http.StatusInternalServerError, "save failed: %v", err)
			return
		}

		allPlugins := h.pluginNamespaces()

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = tmplsessions.SessionInfoCard(meta.ID, meta, allPlugins, true, 0, resolveWindowSize(ctx, h.providers, meta)).Render(ctx, c.Writer)
	}
}

func (h *sessionHandlers) handleArchive() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")

		meta, err := h.sessions.ResolveSession(ctx, id)
		if err != nil {
			c.String(http.StatusNotFound, "not found: %v", err)
			return
		}

		if _, err := h.sessions.ArchiveSession(ctx, meta.ID, "HEAD", ""); err != nil {
			c.String(http.StatusInternalServerError, "archive failed: %v", err)
			return
		}

		c.Header("HX-Redirect", "/ui/sessions/"+meta.ID)
		c.Status(http.StatusOK)
	}
}

func (h *sessionHandlers) handlePluginToggle() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")
		name := strings.ToLower(strings.TrimSpace(c.Param("name")))
		action := c.PostForm("action")

		meta, err := h.sessions.ResolveSession(ctx, id)
		if err != nil {
			c.String(http.StatusNotFound, "not found")
			return
		}
		
		if meta.ArchivedAt != nil {
			c.String(http.StatusConflict, "archived")
			return
		}

		found := false
		for i, p := range meta.Plugins {
			if strings.ToLower(p.Name) == name {
				switch action {
				case "enable":
					meta.Plugins[i].Enabled = true
					meta.Plugins[i].Disabled = false
				
				case "disable":
					meta.Plugins[i].Disabled = true
					meta.Plugins[i].Enabled = false
				
				case "reset":
					meta.Plugins[i].Enabled = false
					meta.Plugins[i].Disabled = false
				
				case "verbose_on":
					meta.Plugins[i].Verbose = true
				
				case "verbose_off":
					meta.Plugins[i].Verbose = false
				}
				found = true
				break
			}
		}
		
		if !found {
			switch action {
			case "enable":
				meta.Plugins = append(meta.Plugins, appsession.PluginConfig{
					Name: name, 
					Enabled: true,
				})
			
			case "disable":
				meta.Plugins = append(meta.Plugins, appsession.PluginConfig{
					Name: name, 
					Disabled: true,
				})
			}
		}

		_ = h.sessions.SaveSession(ctx, meta)

		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		
		_ = tmplsessions.PluginList(id, h.pluginNamespaces(), meta.Plugins, true).Render(ctx, c.Writer)
	}
}

// pluginNamespaces returns the names of all non-builtin registered namespaces.
func (h *sessionHandlers) pluginNamespaces() []string {
	return pluginNamespacesFrom(h.tools)
}

// resolveActiveRef returns the best display branch name from the refs map.
func resolveActiveRef(refs map[string]string, requested string) string {
	if requested != "" && requested != "HEAD" {
		if _, ok := refs[requested]; ok {
			return requested
		}
	}
	if _, ok := refs["main"]; ok {
		return "main"
	}
	for name := range refs {
		if name != "HEAD" {
			return name
		}
	}
	return "HEAD"
}
