package server

import (
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	"github.com/mwantia/forge/pkg/plugins/grpc"
	"github.com/spf13/cobra"
)

func NewPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin [name]",
		Short: "Serve a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			factory := plugins.Get(name)
			if factory == nil {
				return fmt.Errorf("unknown plugin: %s (available: %v)", name, plugins.Names())
			}

			grpc.Serve(factory)
			return nil
		},
	}

	return cmd
}
