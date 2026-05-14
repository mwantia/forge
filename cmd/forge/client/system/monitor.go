package system

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func SystemMonitorCmd(client func() *api.Client) *cobra.Command {
	var logLevel string

	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Stream server logs to the terminal",
		Long: "Connect to the agent's log stream and print lines to stdout as they arrive.\n" +
			"Use --log-level to filter by severity. Press Ctrl+C to disconnect.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ch, err := client().SystemMonitor(ctx, logLevel)
			if err != nil {
				return err
			}
			for line := range ch {
				if line == "" {
					continue
				}
				fmt.Println(line)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Minimum log level to display (trace/debug/info/warn/error)")
	return cmd
}
