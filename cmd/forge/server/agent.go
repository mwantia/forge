package server

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge-sdk/pkg/errors"
	"github.com/mwantia/forge-sdk/pkg/log"
	"github.com/mwantia/forge/internal/agent"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/metrics"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/internal/server"
	"github.com/spf13/cobra"
)

var (
	ConfigFlag string
	OnceFlag   bool
)

func NewAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run forge agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer cancel()

			cfg, err := config.Parse(ConfigFlag)
			if err != nil {
				return fmt.Errorf("unable to parse config: %w", err)
			}

			errs := errors.Errors{}
			sc := container.NewServiceContainer()

			sc.AddTagProcessor(log.NewDefaultLoggerTagProcessor())
			sc.AddTagProcessor(config.NewLoggerTagProcessor(cfg))

			errs.Add(container.Register[*container.ServiceContainer](sc,
				container.AsSingleton(),
				container.WithInstance(sc)))

			errs.Add(container.Register[*registry.PluginRegistry](sc,
				container.AsSingleton()))
			errs.Add(container.Register[*server.Server](sc,
				container.AsSingleton()))
			errs.Add(container.Register[*metrics.Metrics](sc,
				container.AsSingleton()))
			errs.Add(container.Register[*agent.Agent](sc,
				container.AsSingleton()))

			agent, err := container.Resolve[*agent.Agent](ctx, sc)
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			return agent.Serve(OnceFlag, ctx)
		},
	}

	cmd.Flags().StringVar(&ConfigFlag, "config", "forge.hcl", "Defines the configuration path used by this application")
	cmd.Flags().BoolVar(&OnceFlag, "once", false, "Run agent once and exit immediately after startup tests")

	return cmd
}
