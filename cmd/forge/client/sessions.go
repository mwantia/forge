package client

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/client/sessions"
	"github.com/spf13/cobra"
)

func NewSessionsCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage forge sessions",
		Long: "Sessions are content-addressed Merkle DAG chains of immutable messages.\n" +
			"Each session has a HEAD ref and optional named branches. Use these commands\n" +
			"to create, inspect, branch, commit, archive, and restore sessions.",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *api.Client {
		return api.New(httpAddr, httpToken)
	}

	cmd.AddCommand(sessions.SessionsArchiveCmd(client))
	cmd.AddCommand(sessions.SessionsDeleteCmd(client))
	cmd.AddCommand(sessions.SessionsCreateCmd(client))
	cmd.AddCommand(sessions.SessionsCommitCmd(client))
	cmd.AddCommand(sessions.SessionsStatusCmd(client))
	cmd.AddCommand(sessions.SessionsLogCmd(client))
	cmd.AddCommand(sessions.SessionsUpdateCmd(client))

	cmd.AddCommand(sessions.BranchCheckoutCmd(client))
	cmd.AddCommand(sessions.SessionsEditCmd(client))
	cmd.AddCommand(sessions.SessionsShowCmd(client))
	cmd.AddCommand(sessions.SessionsResetCmd(client))
	cmd.AddCommand(sessions.SessionsMessagesCompactCmd(client))
	cmd.AddCommand(sessions.BranchCmd(client))

	return cmd
}
