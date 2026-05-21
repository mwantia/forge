package sessions

import (
	"fmt"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/sessions"
	"github.com/mwantia/forge/internal/service/template"
	"github.com/spf13/cobra"
)

func SessionsArchiveCmd(client func() *v2.ForgeApi) *cobra.Command {
	var ref, rename string
	var renameRandom, deleteAfter bool

	cmd := &cobra.Command{
		Use:   "archive <id>",
		Short: "Archive a session",
		Long: "Walk the named ref (default HEAD), build an archive envelope, and store it\n" +
			"through the resource backend. The session becomes immutable after archiving;\n" +
			"further commits and ref moves return 409.\n\n" +
			"Use -d/--delete to also delete the session after archiving.\n" +
			"Use 'forge sessions clone' to create a new live session from the archive.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if rename != "" && renameRandom {
				return fmt.Errorf("--rename and --rename-random are mutually exclusive")
			}
			name := rename
			if renameRandom {
				name = template.GenerateUniqueName()
			}
			c := client()
			ctx := cmd.Context()
			if err := c.Sessions.Archive(ctx, sessions.SessionsArchiveRequest{
				ID:   args[0],
				Ref:  ref,
				Name: name,
			}); err != nil {
				return err
			}
			fmt.Printf("Session %s archived.\n", args[0])
			if deleteAfter {
				if err := c.Sessions.Delete(ctx, sessions.SessionsDeleteRequest{ID: args[0]}); err != nil {
					return fmt.Errorf("archive succeeded but delete failed: %w", err)
				}
				fmt.Printf("Session %s deleted.\n", args[0])
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&ref, "ref", "", "Branch to archive (default: HEAD)")
	cmd.Flags().StringVar(&rename, "rename", "", "Rename the session before archiving")
	cmd.Flags().BoolVar(&renameRandom, "rename-random", false, "Assign a random name before archiving")
	cmd.Flags().BoolVarP(&deleteAfter, "delete", "d", false, "Delete the session after archiving")

	return cmd
}
