package sessions

import (
	"fmt"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/sessions"
	"github.com/spf13/cobra"
)

func SessionsMessagesCompactCmd(client func() *v2.ForgeApi) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compact <id>",
		Short: "Compact messages in a session by removing tool call entries",
		Long: "Rewrite the active branch removing all tool-call and tool-result messages.\n\n" +
			"The original chain is left intact as orphaned objects; run 'forge system gc'\n" +
			"to reclaim the space.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := client().Sessions.Compact(cmd.Context(), sessions.SessionsCompactRequest{
				SessionID: args[0],
			})
			if err != nil {
				return err
			}
			fmt.Printf("Compacted: %d → %d messages (%d deleted)\n",
				result.Before, result.After, result.Deleted)
			return nil
		},
	}
	return cmd
}
