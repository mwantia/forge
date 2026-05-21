package sessions

import (
	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/sessions"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsUpdateCmd(client func() *v2.ForgeApi) *cobra.Command {
	var name, title, description, model string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update session metadata (name, title, description, model)",
		Long: "Patch mutable fields on a session. Only the flags you explicitly pass are\n" +
			"applied; omitted flags leave the existing values unchanged.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := sessions.SessionsUpdateRequest{ID: args[0]}
			if cmd.Flags().Changed("name") {
				req.Name = name
			}
			if cmd.Flags().Changed("title") {
				req.Title = title
			}
			if cmd.Flags().Changed("description") {
				req.Description = description
			}
			if cmd.Flags().Changed("model") {
				req.Model = model
			}
			resp, err := client().Sessions.Update(cmd.Context(), req)
			if err != nil {
				return err
			}
			helpers.PrintSession(resp.Session, false)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "New session name")
	cmd.Flags().StringVar(&title, "title", "", "New session title")
	cmd.Flags().StringVar(&description, "description", "", "New session description")
	cmd.Flags().StringVar(&model, "model", "", "New model (format: provider/model)")

	return cmd
}
