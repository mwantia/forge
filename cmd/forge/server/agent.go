package server

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge-sdk/pkg/errors"
	"github.com/mwantia/forge-sdk/pkg/log"
	"github.com/mwantia/forge/internal/agent"
	"github.com/mwantia/forge/internal/channel"
	"github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/metrics"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/internal/sandbox"
	"github.com/mwantia/forge/internal/server"
	"github.com/mwantia/forge/internal/session"
	"github.com/mwantia/forge/internal/storage"
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
			errs.Add(container.Register[*channel.ChannelDispatcher](sc,
				container.AsSingleton()))
			errs.Add(container.Register[*sandbox.SandboxManager](sc,
				container.AsSingleton()))
			errs.Add(container.Register[*session.SessionManager](sc,
				container.AsSingleton()))
			errs.Add(container.Register[*server.Server](sc,
				container.AsSingleton()))
			errs.Add(container.Register[*metrics.Metrics](sc,
				container.AsSingleton()))
			errs.Add(container.Register[*agent.Agent](sc,
				container.AsSingleton()))

			errs.Add(container.Register[*storage.StorageBackendInjector](sc,
				container.AsSingleton(),
				container.With[storage.Backend]()))

			// No idea why we have to manually resolve it here
			container.Resolve[storage.Backend](ctx, sc)

			agent, err := container.Resolve[*agent.Agent](ctx, sc)
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			if err := agent.Serve(OnceFlag, ctx); err != nil {
				return err
			}

			return sc.Cleanup(ctx)
		},
	}

	cmd.Flags().StringVar(&ConfigFlag, "config", "forge.hcl", "Defines the configuration path used by this application")
	cmd.Flags().BoolVar(&OnceFlag, "once", false, "Run agent once and exit immediately after startup tests")

	return cmd
}
