package memory

import "github.com/mwantia/forge-sdk/pkg/plugins"

// ToolsDefinitions are registered under the "memory" namespace at Init.
var ToolsDefinitions = []plugins.ToolDefinition{
	{
		Name:        "store_resource",
		Description: `Persist a piece of context (text plus optional metadata) into long-term memory so it can be retrieved by later turns or sessions.`,
		Tags:        []string{"memory", "store"},
		Annotations: plugins.ToolAnnotations{
			CostHint: plugins.ToolCostCheap,
			System: `
Persist information that future turns or future sessions may need:
user preferences, project facts, decisions made, things the user
explicitly asked to remember. Skip transient turn details — those
already live in message history. Pass structured metadata when it
helps later filter calls (e.g. {"kind":"preference"}).
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"content":   {Type: "string", Description: "The text content to store."},
				"namespace": {Type: "string", Description: "Optional namespace. Defaults to the caller session ID, then to the configured default namespace."},
				"metadata":  {Type: "object", Description: "Optional structured metadata attached to the stored resource."},
			},
			Required: []string{"content"},
		},
	},
	{
		Name:        "retrieve_resources",
		Description: `Retrieve previously stored context resources by semantic similarity to the provided query.`,
		Tags:        []string{"memory", "retrieve"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
			System: `
Semantic similarity search over previously stored resources. Reach for
this when the user references prior knowledge ("the thing we decided
last week", "my preferences") that isn't in the current message
history. Frame the query as the natural-language meaning to recall,
not as keyword soup. Default limit of 5 is usually plenty.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"query":     {Type: "string", Description: "The search query."},
				"limit":     {Type: "integer", Description: "Maximum number of results to return. Defaults to 5."},
				"namespace": {Type: "string", Description: "Optional namespace. Defaults to the caller session ID, then to the configured default namespace."},
				"filter":    {Type: "object", Description: "Optional metadata filter applied by the memory backend."},
			},
			Required: []string{"query"},
		},
	},
}
