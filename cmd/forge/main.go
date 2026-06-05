package main

import (
	"io"
	"log"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/cmd/forge/client"
	cliserver "github.com/mwantia/forge/cmd/forge/server"
	"github.com/mwantia/forge/internal/application/agent"
	appapprovals "github.com/mwantia/forge/internal/application/approvals"
	appevent "github.com/mwantia/forge/internal/application/event"
	apppipeline "github.com/mwantia/forge/internal/application/pipeline"
	appplugins "github.com/mwantia/forge/internal/application/plugins"
	appresource "github.com/mwantia/forge/internal/application/resource"
	appsandbox "github.com/mwantia/forge/internal/application/sandbox"
	appsession "github.com/mwantia/forge/internal/application/session"
	appsystem "github.com/mwantia/forge/internal/application/system"
	apptools "github.com/mwantia/forge/internal/application/tools"
	appui "github.com/mwantia/forge/internal/application/ui"
	_ "github.com/mwantia/forge/internal/config"
	inframetrics "github.com/mwantia/forge/internal/infrastructure/metrics"
	infraserver "github.com/mwantia/forge/internal/infrastructure/server"
	infrastorage "github.com/mwantia/forge/internal/infrastructure/storage"
	infratemplate "github.com/mwantia/forge/internal/infrastructure/template"
	forgelog "github.com/mwantia/forge/internal/log"
	"github.com/spf13/cobra"
)

// Ensure init() registrations run for all service packages.
var (
	_ *agent.Agent                   = nil
	_ *appui.UIService               = nil
	_ *appapprovals.ApprovalService  = nil
	_ *appevent.EventService         = nil
	_ *inframetrics.MetricsService   = nil
	_ *apppipeline.PipelineService   = nil
	_ *appplugins.PluginsService     = nil
	_ *appresource.ResourceService   = nil
	_ *appsandbox.SandboxService     = nil
	_ *infraserver.ServerService     = nil
	_ *appsession.SessionService     = nil
	_ *infrastorage.StorageService   = nil
	_ *appsystem.SystemService       = nil
	_ *infratemplate.TemplateService = nil
	_ *apptools.ToolsService         = nil
)

var (
	// Root flags
	LogLevelFlag   string
	NoLogColorFlag bool
	// Sandbox flags
	SandboxModelFlag       string
	SandboxTemperatureFlag float64
	SandboxMaxTokenFlag    int
)

func main() {
	cmd := &cobra.Command{
		Use:   "forge",
		Short: "System for forging",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			lvl := hclog.LevelFromString(LogLevelFlag)
			forgelog.Bootstrap(lvl, !NoLogColorFlag)
			log.SetOutput(io.Discard)

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&LogLevelFlag, "log-level", "info", "Defines the threshold for the logger")
	cmd.PersistentFlags().BoolVar(&NoLogColorFlag, "no-color", false, "Disables colored command output")

	cmd.AddCommand(cliserver.NewAgentCommand())
	cmd.AddCommand(cliserver.NewPluginCommand())
	cmd.AddCommand(client.NewSessionsCommand())
	cmd.AddCommand(client.NewResourceCommand())
	cmd.AddCommand(client.NewContextsCommand())
	cmd.AddCommand(client.NewEventsCommand())
	cmd.AddCommand(client.NewSystemCommand())
	cmd.AddCommand(client.NewApprovalsCommand())

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
