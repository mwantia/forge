package messages

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsMessagesViewCmd(client func() *api.Client) *cobra.Command {
	var noRender bool

	cmd := &cobra.Command{
		Use:   "view <session-id> <message-id>",
		Short: "View the content of a single message",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			msg, err := client().GetMessage(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}

			var sb strings.Builder
			sb.WriteString("---\n")
			fmt.Fprintf(&sb, "Hash:    %s\n", msg.Hash)
			fmt.Fprintf(&sb, "Role:    %s\n", msg.Role)
			fmt.Fprintf(&sb, "Created: %s\n", msg.CreatedAt.Format(time.DateTime))
			sb.WriteString("---\n")
			if msg.Content != "" {
				sb.WriteString("\n")
				sb.WriteString(msg.Content)
			}
			for _, tc := range msg.ToolCalls {
				sb.WriteString("\n---\n")
				if tc.Result != "" {
					fmt.Fprintf(&sb, "tool_result  %s\n", tc.Name)
					if tc.IsError {
						sb.WriteString("(error)\n")
					}
					sb.WriteString(tc.Result)
				} else {
					fmt.Fprintf(&sb, "tool_call  %s\n", tc.Name)
					if len(tc.Arguments) > 0 {
						b, _ := json.MarshalIndent(tc.Arguments, "", "  ")
						sb.Write(b)
						sb.WriteByte('\n')
					}
				}
			}
			fmt.Println(helpers.RenderMarkdown(sb.String(), noRender))
			return nil
		},
	}

	cmd.Flags().BoolVar(&noRender, "no-render", false, "Print raw content without markdown rendering")
	return cmd
}
