package sessions

import (
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/internal/service/template"
	"github.com/spf13/cobra"
)

func SessionsArchiveCmd(client func() *api.Client) *cobra.Command {
	var ref, rename string
	var renameRandom bool

	cmd := &cobra.Command{
		Use:   "archive <id>",
		Short: "Archive a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if rename != "" && renameRandom {
				return fmt.Errorf("--rename and --rename-random are mutually exclusive")
			}
			name := rename
			if renameRandom {
				name = template.GenerateUniqueName()
			}
			if err := client().ArchiveSession(cmd.Context(), args[0], ref, name); err != nil {
				return err
			}
			fmt.Printf("Session %s archived.\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&ref, "ref", "", "Branch to archive (default: HEAD)")
	cmd.Flags().StringVar(&rename, "rename", "", "Rename the session before archiving")
	cmd.Flags().BoolVar(&renameRandom, "rename-random", false, "Assign a random name before archiving")

	return cmd
}
