package sessions

import (
	"fmt"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/sessions"
	"github.com/spf13/cobra"
)

func SessionsDeleteCmd(client func() *v2.ForgeApi) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an archived session",
		Long: "Permanently deletes an archived session and all its stored data.\n" +
			"Only archived sessions may be deleted. Run 'forge sessions archive <id>'\n" +
			"first to preserve the session history as a resource before removing it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := client().Sessions.Delete(cmd.Context(), sessions.SessionsDeleteRequest{ID: args[0]}); err != nil {
				return err
			}
			fmt.Printf("Session %s deleted.\n", args[0])
			return nil
		},
	}
}
