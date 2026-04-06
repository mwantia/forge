package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/hashicorp/go-hclog"
	wlog "github.com/mwantia/forge-sdk/pkg/log"
	"github.com/mwantia/forge/cmd/forge/client"
	"github.com/mwantia/forge/cmd/forge/server"
	"github.com/spf13/cobra"
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
				Output:      wlog.LogWrapper(os.Stdout, !NoLogColorFlag),
				JSONFormat:  false,
			})
			hclog.SetDefault(logger)
			log.SetOutput(io.Discard)

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&LogLevelFlag, "log-level", "info", "Defines the threshold for the logger")
	cmd.PersistentFlags().BoolVar(&NoLogColorFlag, "no-color", false, "Disables colored command output")

	cmd.AddCommand(server.NewAgentCommand())
	cmd.AddCommand(server.NewPluginCommand())
	cmd.AddCommand(client.NewSessionsCommand())
	cmd.AddCommand(client.NewChannelsCommand())

	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
