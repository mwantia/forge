package pipeline

import "github.com/mwantia/forge-sdk/pkg/plugins"

var ToolsDefinitions = []plugins.ToolDefinition{
	{
		Name:        "commit_session",
		Description: `Send a message to a session and wait for the full response. Blocks until the sub-session finishes processing.`,
		Tags:        []string{"session", "commit"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostModerate,
			System: `
Synchronous: blocks until the target session finishes its full tool-loop. Use sparingly — every commit is a nested LLM run with its own token cost.
Frame the message tightly so the sub-session can answer and return without further round-trips.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id": {Type: "string", Description: "The ID of the session to commit to."},
				"content":    {Type: "string", Description: "The message content to send."},
			},
			Required: []string{"session_id", "content"},
		},
	},
}
