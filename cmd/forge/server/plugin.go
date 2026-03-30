package server

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge-sdk/pkg/plugins/grpc"
	"github.com/spf13/cobra"
)

func NewPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin [name]",
		Short: "Serve a plugin or list available plugins",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				entries := plugins.List()
				if len(entries) == 0 {
					fmt.Println("No plugins available.")
					return nil
				}

				// Calculate column width from longest name
				maxLen := 4 // len("NAME")
				for _, e := range entries {
					if len(e.Name) > maxLen {
						maxLen = len(e.Name)
					}
				}

				fmt.Printf("Available plugins:\n\n")
				fmt.Printf("  %-*s  %s\n", maxLen, "NAME", "DESCRIPTION")
				for _, e := range entries {
					fmt.Printf("  %-*s  %s\n", maxLen, e.Name, e.Description)
				}
				fmt.Printf("\nUse 'forge plugin <name>' to serve a plugin.\n")
				return nil
			}

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
