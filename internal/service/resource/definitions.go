package resource

import "github.com/mwantia/forge-sdk/pkg/plugins"

// ToolsDefinitions are registered under the "resource" namespace at Init.
// LLM-visible names are prefixed: resource__store, resource__recall,
// resource__forget (docs/03 §5.3).
var ToolsDefinitions = []plugins.ToolDefinition{
	{
		Name:        "store",
		Description: `Persist a piece of context (text plus optional metadata) into long-term memory so it can be retrieved by later turns or sessions.`,
		Tags:        []string{"resource", "store"},
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
		Name:        "recall",
		Description: `Retrieve previously stored context resources by semantic similarity to the provided query.`,
		Tags:        []string{"resource", "recall"},
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
				"filter":    {Type: "object", Description: "Optional metadata filter applied by the resource backend."},
			},
			Required: []string{"query"},
		},
	},
	{
		Name:        "forget",
		Description: `Delete a previously stored resource by ID.`,
		Tags:        []string{"resource", "forget"},
		Annotations: plugins.ToolAnnotations{
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
			System: `
Use when the user asks you to remove a stored memory, or when a
prior recall surfaced information that is no longer correct. Pair
with recall first to find the right ID; do not loop forget over
many IDs without confirmation.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"id":        {Type: "string", Description: "The ID of the resource to delete."},
				"namespace": {Type: "string", Description: "Optional namespace. Defaults to the caller session ID, then to the configured default namespace."},
			},
			Required: []string{"id"},
		},
	},
}
