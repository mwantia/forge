package sessions

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/sessions"
	"github.com/mwantia/forge/cmd/forge/helpers"
	"github.com/spf13/cobra"
)

func SessionsShowCmd(client func() *v2.ForgeApi) *cobra.Command {
	var noRender bool

	cmd := &cobra.Command{
		Use:   "show <session-id> <message-id>",
		Short: "Show the content of a single message",
		Long: "Fetch a single message by session ID/name and message hash (or prefix) and\n" +
			"print its content, role, and timestamp. Tool call arguments and results are\n" +
			"rendered inline. Pass --no-render to skip markdown rendering.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().Sessions.GetMessage(cmd.Context(), sessions.SessionsGetMessageRequest{
				SessionID: args[0],
				MessageID: args[1],
			})
			if err != nil {
				return err
			}
			msg := resp.Message

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
