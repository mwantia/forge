package events

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mwantia/forge-sdk/pkg/api"
	"github.com/spf13/cobra"
)

func EventsFireCmd(client func() *api.Client) *cobra.Command {
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

			return printFireResponse(resp)
		},
	}

	cmd.Flags().StringVar(&payload, "payload", "", "Payload string or JSON")
	cmd.Flags().StringVar(&payloadFile, "payload-file", "", "Path to a file whose contents are the payload")
	cmd.Flags().StringVar(&ref, "ref", "", "Branch base override")

	return cmd
}
