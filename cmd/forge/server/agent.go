package server

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/mwantia/fabric/pkg/container"
	"github.com/mwantia/forge/internal/agent"
	forgeconfig "github.com/mwantia/forge/internal/config"
	"github.com/mwantia/forge/internal/service/template"
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

			tmpl, err := container.Resolve[template.TemplateRenderer](ctx)
			if err != nil {
				return fmt.Errorf("failed to resolve template renderer: %w", err)
			}

			cfg, err := forgeconfig.Parse(ConfigFlag, tmpl.Base())
			if err != nil {
				return fmt.Errorf("unable to parse config: %w", err)
			}

			forgeconfig.SetConfig(cfg, tmpl.Base())

			agent, err := container.Resolve[*agent.Agent](ctx)
			if err != nil {
				return fmt.Errorf("failed to resolve agent: %w", err)
			}

			if err := agent.Serve(OnceFlag, ctx); err != nil {
				return err
			}

			return agent.Cleanup(ctx)
		},
	}

	cmd.Flags().StringVar(&ConfigFlag, "config", "forge.hcl", "Defines the configuration path used by this application")
	cmd.Flags().BoolVar(&OnceFlag, "once", false, "Run agent once and exit immediately after startup tests")

	return cmd
}
