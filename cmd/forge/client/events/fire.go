package events

import (
	"encoding/json"
	"fmt"
	"os"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/events"
	"github.com/spf13/cobra"
)

func EventsFireCmd(client func() *v2.ForgeApi) *cobra.Command {
	var payload, payloadFile, ref string

	cmd := &cobra.Command{
		Use:   "fire <id>",
		Short: "Fire an event",
		Long: "Trigger an event endpoint immediately, optionally supplying a JSON payload.\n\n" +
			"The payload is forwarded to the event's pipeline session as context.\n" +
			"Use --payload for an inline string/JSON value or --payload-file to read from a file.",
		Args: cobra.ExactArgs(1),
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

			resp, err := client().Events.Fire(cmd.Context(), events.EventsFireRequest{
				ID:      args[0],
				Ref:     ref,
				Payload: body,
			})
			if err != nil {
				return err
			}

			return printFireResponse(resp.Fire)
		},
	}

	cmd.Flags().StringVar(&payload, "payload", "", "Payload string or JSON")
	cmd.Flags().StringVar(&payloadFile, "payload-file", "", "Path to a file whose contents are the payload")
	cmd.Flags().StringVar(&ref, "ref", "", "Branch base override")

	return cmd
}
