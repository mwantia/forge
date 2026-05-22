package events

import (
	"encoding/json"
	"fmt"
	"os"

	v2 "github.com/mwantia/forge-sdk/pkg/api/v2"
	"github.com/mwantia/forge-sdk/pkg/api/v2/events"
	"github.com/spf13/cobra"
)

func EventsPushCmd(client func() *v2.ForgeApi) *cobra.Command {
	var payload, payloadFile, ref string
	var async bool

	cmd := &cobra.Command{
		Use:   "push <id>",
		Short: "Push an event",
		Long: "Trigger an event endpoint immediately, optionally supplying a JSON payload.\n\n" +
			"The payload is forwarded to the event's pipeline session as context.\n" +
			"Use --payload for an inline string/JSON value or --payload-file to read from a file.\n\n" +
			"By default the call blocks until the pipeline finishes and prints the response.\n" +
			"Pass --async to dispatch in the background and return immediately.",
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

			resp, err := client().Events.Push(cmd.Context(), events.EventsPushRequest{
				ID:      args[0],
				Ref:     ref,
				Payload: body,
				Async:   async,
			})
			if err != nil {
				return err
			}

			return printPushResponse(resp.Push)
		},
	}

	cmd.Flags().StringVar(&payload, "payload", "", "Payload string or JSON")
	cmd.Flags().StringVar(&payloadFile, "payload-file", "", "Path to a file whose contents are the payload")
	cmd.Flags().StringVar(&ref, "ref", "", "Branch base override")
	cmd.Flags().BoolVar(&async, "async", false, "Dispatch in background and return immediately (skips content output)")

	return cmd
}
