package client

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/client/sessions"
	"github.com/mwantia/forge/cmd/forge/client/sessions/branch"
	"github.com/mwantia/forge/cmd/forge/client/sessions/messages"
	"github.com/mwantia/forge/cmd/forge/client/sessions/system"
	"github.com/spf13/cobra"
)

func NewSessionsCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage forge sessions",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *api.Client {
		return api.New(httpAddr, httpToken)
	}

	cmd.AddCommand(sessions.SessionsArchiveCmd(client))
	cmd.AddCommand(sessions.SessionsCreateCmd(client))
	cmd.AddCommand(sessions.SessionsDispatchCmd(client))
	cmd.AddCommand(sessions.SessionsGetCmd(client))
	cmd.AddCommand(sessions.SessionsListCmd(client))
	cmd.AddCommand(sessions.SessionsLogCmd(client))
	cmd.AddCommand(sessions.SessionsUpdateCmd(client))

	cmd.AddCommand(SessionsMessagesCmd(client))
	cmd.AddCommand(SessionsBranchCmd(client))
	cmd.AddCommand(SessionsSystemCmd(client))

	return cmd
}

func SessionsMessagesCmd(client func() *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Manage messages in a session",
	}

	cmd.AddCommand(messages.SessionsMessagesViewCmd(client))
	cmd.AddCommand(messages.SessionsMessagesCompactCmd(client))
	cmd.AddCommand(messages.SessionsMessagesEditCmd(client))

	return cmd
}

func SessionsBranchCmd(client func() *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Manage session branches",
	}

	cmd.AddCommand(branch.BranchListCmd(client))
	cmd.AddCommand(branch.BranchCreateCmd(client))
	cmd.AddCommand(branch.BranchCheckoutCmd(client))
	cmd.AddCommand(branch.BranchDeleteCmd(client))
	cmd.AddCommand(branch.BranchRenameCmd(client))

	return cmd
}

func SessionsSystemCmd(client func() *api.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Manage the system message for a session",
	}

	cmd.AddCommand(system.SystemViewCmd(client))
	cmd.AddCommand(system.SystemEditCmd(client))
	cmd.AddCommand(system.SystemResetCmd(client))

	return cmd
}
