package ui

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/v2/pkg/container"
	approot "github.com/mwantia/forge/internal/application"
	apppipeline "github.com/mwantia/forge/internal/application/pipeline"
	appsession "github.com/mwantia/forge/internal/application/session"
	domprovider "github.com/mwantia/forge/internal/domain/provider"
	domresource "github.com/mwantia/forge/internal/domain/resource"
	domtool "github.com/mwantia/forge/internal/domain/tool"
	infraserver "github.com/mwantia/forge/internal/infrastructure/server"
	"github.com/mwantia/forge/internal/application/ui/templates/layout"
)

type UIService struct {
	approot.UnimplementedService

	router    infraserver.HttpRouter         `fabric:"inject"`
	sessions  *appsession.SessionService     `fabric:"inject"`
	tools     domtool.ToolsRegistar          `fabric:"inject"`
	providers domprovider.ProviderRegistar   `fabric:"inject"`
	pipeline  apppipeline.PipelineCommitter  `fabric:"inject"`
	renderer  apppipeline.PipelineRenderer   `fabric:"inject"`
	resources domresource.ResourceRegistar   `fabric:"inject"`

	logger hclog.Logger `fabric:"logger=ui"`
}

func init() {
	container.MustRegister[*UIService](container.AsSingleton())
}

func (u *UIService) PostInit(_ context.Context) error {
	g := u.router.UIGroup("/ui")

	g.Use(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/ui/assets/") {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	})

	g.GET("", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/sessions")
	})

	sess := &sessionHandlers{
		sessions:  u.sessions,
		tools:     u.tools,
		renderer:  u.renderer,
		providers: u.providers,
	}
	g.GET("/sessions", sess.handleList())
	g.POST("/sessions", sess.handleCreate())
	g.GET("/sessions/new", sess.handleNew())
	g.GET("/sessions/:id", sess.handleDetail())
	g.DELETE("/sessions/:id", sess.handleDelete())
	g.POST("/sessions/:id/archive", sess.handleArchive())
	g.GET("/sessions/:id/thread", sess.handleThread())
	g.GET("/sessions/:id/node", sess.handleNodePanel())
	g.GET("/sessions/:id/edit", sess.handleEdit())
	g.PATCH("/sessions/:id", sess.handleUpdate())
	g.PATCH("/sessions/:id/plugins/:name", sess.handlePluginToggle())

	refs := &refHandlers{
		sessions: u.sessions,
	}
	g.GET("/sessions/:id/refs", refs.handlePanel())
	g.POST("/sessions/:id/refs", refs.handleCreate())
	g.DELETE("/sessions/:id/refs/:ref", refs.handleDelete())
	g.POST("/sessions/:id/refs/:ref/checkout", refs.handleCheckout())

	pipe := &pipelineHandlers{
		pipeline: u.pipeline,
	}
	g.POST("/sessions/:id/commit", pipe.handleCommit())

	stream := &streamHandlers{
		sessions:  u.sessions,
		tools:     u.tools,
		renderer:  u.renderer,
		pipeline:  u.pipeline,
		providers: u.providers,
	}
	g.GET("/sessions/:id/stream", stream.handleStream())

	dag := &dagHandlers{
		sessions: u.sessions,
	}
	g.GET("/sessions/:id/dag", dag.handleFull())
	g.GET("/sessions/:id/dag/mini", dag.handleMini())

	res := &resourceHandlers{
		resources: u.resources,
	}
	g.GET("/resources", res.handleList())

	layout.SetAssetVersion(AssetVersion)

	serviceWorker := serviceWorkerJSWithVersion()
	fileServer := http.FileServer(staticFS())

	g.GET("/assets/*filepath", func(c *gin.Context) {
		filepath := c.Param("filepath")

		switch filepath {
		case "/sw.js":
			c.Header("Service-Worker-Allowed", "/ui/")
			c.Header("Cache-Control", "no-cache")
			c.Data(http.StatusOK, "application/javascript; charset=utf-8", serviceWorker)
		
		case "/manifest.json":
			c.Header("Cache-Control", "no-cache")
			c.Request.URL.Path = filepath
			c.Request.URL.RawQuery = ""
			fileServer.ServeHTTP(c.Writer, c.Request)
		
		default:
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			c.Request.URL.Path = filepath
			c.Request.URL.RawQuery = ""
			fileServer.ServeHTTP(c.Writer, c.Request)
		}
	})

	u.logger.Debug("Initialized ui service")
	return nil
}
