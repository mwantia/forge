package sessions

import (
	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsGetCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get session details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			meta, err := client().GetSession(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			helpers.PrintSession(meta)
			return nil
		},
	}
}
