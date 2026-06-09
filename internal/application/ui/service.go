package ui

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/fabric/v2/pkg/container"
	approot "github.com/mwantia/forge/internal/application"
	apppipeline "github.com/mwantia/forge/internal/application/pipeline"
	appsession "github.com/mwantia/forge/internal/application/session"
	infraserver "github.com/mwantia/forge/internal/infrastructure/server"
)

type UIService struct {
	approot.UnimplementedService

	router   infraserver.HttpRouter         `fabric:"inject"`
	sessions *appsession.SessionService     `fabric:"inject"`
	pipeline apppipeline.PipelineCommitter  `fabric:"inject"`
	renderer apppipeline.PipelineRenderer   `fabric:"inject"`

	logger hclog.Logger `fabric:"logger=ui"`
}

func init() {
	container.MustRegister[*UIService](container.AsSingleton())
}

func (u *UIService) PostInit(_ context.Context) error {
	g := u.router.UIGroup("/ui")

	g.GET("", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/sessions")
	})

	sess := &sessionHandlers{
		sessions: u.sessions,
		renderer: u.renderer,
	}
	g.GET("/sessions", sess.handleList())
	g.POST("/sessions", sess.handleCreate())
	g.GET("/sessions/:id", sess.handleDetail())
	g.DELETE("/sessions/:id", sess.handleDelete())

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

	dag := &dagHandlers{
		sessions: u.sessions,
	}
	g.GET("/sessions/:id/dag", dag.handleFull())
	g.GET("/sessions/:id/dag/mini", dag.handleMini())

	g.StaticFS("/assets", staticFS())

	u.logger.Debug("Initialized ui service...")

	return nil
}
