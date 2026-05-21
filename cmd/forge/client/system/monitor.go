package system

import (
	"fmt"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/system"
	"github.com/spf13/cobra"
)

func SystemMonitorCmd(client func() *v2.ForgeApi) *cobra.Command {
	var logLevel string

	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Stream server logs to the terminal",
		Long: "Connect to the agent's log stream and print lines to stdout as they arrive.\n" +
			"Use --log-level to filter by severity. Press Ctrl+C to disconnect.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			resp, err := client().System.Monitor(ctx, system.SystemMonitorRequest{Level: logLevel})
			if err != nil {
				return err
			}
			for line := range resp.Lines {
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
