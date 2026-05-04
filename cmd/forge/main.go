//go:generate swag init -g cmd/forge/main.go -o docs --parseDependency --parseInternal

package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge/cmd/forge/client"
	cliserver "github.com/mwantia/forge/cmd/forge/server"
	"github.com/mwantia/forge/internal/agent"
	flog "github.com/mwantia/forge/internal/log"
	"github.com/mwantia/forge/internal/service/metrics"
	"github.com/mwantia/forge/internal/service/pipeline"
	"github.com/mwantia/forge/internal/service/plugins"
	"github.com/mwantia/forge/internal/service/resource"
	"github.com/mwantia/forge/internal/service/sandbox"
	"github.com/mwantia/forge/internal/service/server"
	"github.com/mwantia/forge/internal/service/session"
	"github.com/mwantia/forge/internal/service/storage"
	"github.com/mwantia/forge/internal/service/template"
	"github.com/mwantia/forge/internal/service/tools"
	"github.com/spf13/cobra"
)

// Ensure init() registrations run for all service packages.
var (
	_ *agent.Agent              = nil
	_ *metrics.MetricsService   = nil
	_ *pipeline.PipelineService = nil
	_ *plugins.PluginsService   = nil
	_ *resource.ResourceService = nil
	_ *sandbox.SandboxService   = nil
	_ *server.ServerService     = nil
	_ *session.SessionService   = nil
	_ *storage.StorageService   = nil
	_ *template.TemplateService = nil
	_ *tools.ToolsService       = nil
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
			logger := hclog.New(&hclog.LoggerOptions{
				Name:        "forge",
				DisableTime: true,
				Level:       hclog.LevelFromString(LogLevelFlag),
				Output:      flog.LogWrapper(os.Stdout, !NoLogColorFlag),
				JSONFormat:  false,
			})
			hclog.SetDefault(logger)
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
	cmd.AddCommand(client.NewReplayCommand())
	//cmd.AddCommand(client.NewChannelsCommand())

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
