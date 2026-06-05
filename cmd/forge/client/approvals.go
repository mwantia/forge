package client

import (
	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge/cmd/forge/client/approvals"
	"github.com/spf13/cobra"
)

func NewApprovalsCommand() *cobra.Command {
	var httpAddr, httpToken string
	cmd := &cobra.Command{
		Use:   "approvals",
		Short: "Manage pending tool-call approvals",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *v2.ForgeApi {
		return v2.NewApi(httpAddr, httpToken)
	}

	cmd.AddCommand(approvals.ApprovalsListCmd(client))
	cmd.AddCommand(approvals.ApprovalsInspectCmd(client))
	cmd.AddCommand(approvals.ApprovalsAllowCmd(client))
	cmd.AddCommand(approvals.ApprovalsDenyCmd(client))
	cmd.AddCommand(approvals.ApprovalsWatchCmd(client))

	return cmd
}
