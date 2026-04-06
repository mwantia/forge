package client

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/mwantia/forge/internal/channel"
	"github.com/spf13/cobra"
)

func NewChannelsCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Manage channel-to-session bindings",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *ForgeClient { return resolveClient(httpAddr, httpToken) }

	cmd.AddCommand(newChannelsListCmd(client))
	cmd.AddCommand(newChannelsBindCmd(client))
	cmd.AddCommand(newChannelsClearCmd(client))

	return cmd
}

func newChannelsListCmd(client func() *ForgeClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list <plugin>",
		Short: "List all channel bindings for a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plugin := args[0]

			var resp struct {
				Bindings map[string]*channel.ChannelBinding `json:"bindings"`
			}
			if err := client().get(fmt.Sprintf("/v1/channels/%s/bindings", plugin), &resp); err != nil {
				return err
			}

			if len(resp.Bindings) == 0 {
				fmt.Printf("No bindings for plugin %q.\n", plugin)
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "CHANNEL ID\tSESSION\tBOUND AT")
			for channelID, b := range resp.Bindings {
				fmt.Fprintf(w, "%s\t%s\t%s\n",
					channelID,
					b.SessionName,
					b.BoundAt.Format(time.DateTime),
				)
			}
			return w.Flush()
		},
	}
}

func newChannelsBindCmd(client func() *ForgeClient) *cobra.Command {
	return &cobra.Command{
		Use:   "bind <plugin> <channel-id> <session-name>",
		Short: "Bind a channel to a session",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			plugin, channelID, sessionName := args[0], args[1], args[2]

			body := map[string]string{
				"channel_id":   channelID,
				"session_name": sessionName,
			}
			var resp struct {
				Binding *channel.ChannelBinding `json:"binding"`
			}
			if err := client().post(fmt.Sprintf("/v1/channels/%s/bind", plugin), body, &resp); err != nil {
				return err
			}

			fmt.Printf("Bound channel %s to session %q (id: %s)\n",
				channelID, resp.Binding.SessionName, resp.Binding.SessionID)
			return nil
		},
	}
}

func newChannelsClearCmd(client func() *ForgeClient) *cobra.Command {
	return &cobra.Command{
		Use:   "clear <plugin> <channel-id>",
		Short: "Unbind a channel from its session",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			plugin, channelID := args[0], args[1]
			if err := client().delete(fmt.Sprintf("/v1/channels/%s/bind/%s", plugin, channelID)); err != nil {
				return err
			}
			fmt.Printf("Unbound channel %s from plugin %q\n", channelID, plugin)
			return nil
		},
	}
}
