package client

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func NewEventsCommand() *cobra.Command {
	var httpAddr, httpToken string

	cmd := &cobra.Command{
		Use:   "events",
		Short: "Manage forge event endpoints",
	}

	cmd.PersistentFlags().StringVar(&httpAddr, "http-addr", "", "Address of the forge agent (env: FORGE_HTTP_ADDR)")
	cmd.PersistentFlags().StringVar(&httpToken, "http-token", "", "Auth token for the forge agent (env: FORGE_HTTP_TOKEN)")

	client := func() *api.Client { return api.New(httpAddr, httpToken) }

	cmd.AddCommand(newEventsListCmd(client))
	cmd.AddCommand(newEventsShowCmd(client))
	cmd.AddCommand(newEventsFireCmd(client))

	return cmd
}

func newEventsListCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured events",
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := client().ListEvents(cmd.Context())
			if err != nil {
				return err
			}
			if len(events) == 0 {
				fmt.Println("No events configured.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSESSION\tDESCRIPTION")
			for _, ev := range events {
				fmt.Fprintf(w, "%s\t%s\t%s\n", ev.ID, ev.Session, ev.Description)
			}
			return w.Flush()
		},
	}
}

func newEventsShowCmd(client func() *api.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show event details and live queue state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ev, err := client().GetEvent(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printJSON(ev)
		},
	}
}

func newEventsFireCmd(client func() *api.Client) *cobra.Command {
	var payload, payloadFile, ref string

	cmd := &cobra.Command{
		Use:   "fire <id>",
		Short: "Fire an event",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var body any

			switch {
			case payloadFile != "":
				data, err := os.ReadFile(payloadFile)
				if err != nil {
					return fmt.Errorf("read payload file: %w", err)
				}
				var v any
				if json.Unmarshal(data, &v) == nil {
					body = v
				} else {
					body = string(data)
				}
			case payload != "":
				var v any
				if json.Unmarshal([]byte(payload), &v) == nil {
					body = v
				} else {
					body = payload
				}
			}

			resp, err := client().FireEvent(cmd.Context(), args[0], body, ref)
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}

	cmd.Flags().StringVar(&payload, "payload", "", "Payload string or JSON")
	cmd.Flags().StringVar(&payloadFile, "payload-file", "", "Path to a file whose contents are the payload")
	cmd.Flags().StringVar(&ref, "ref", "", "Branch base override")

	return cmd
}
