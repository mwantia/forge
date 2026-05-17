package resource

import "github.com/mwantia/forge-sdk/pkg/plugins"

// ToolsDefinitions are registered under the "resource" namespace at Init.
// LLM-visible names are prefixed: resource__store, resource__recall,
// resource__forget.
var ToolsDefinitions = []plugins.ToolDefinition{
	{
		Name: "store",
		Description: `Persist a piece of context into long-term memory for retrieval across turns or sessions.

type controls where the resource lives:
  memory    — facts, preferences, decisions, recurring constraints; stored under the current session namespace.
  reference — external links, cited documents, API specs, code snippets; stored per-session.
  online    — web-fetched or time-sensitive content that may go stale; stored per-session. Set metadata.expires_at (RFC3339) to mark freshness.
  global    — agent-wide facts shared across all sessions (e.g. user identity, long-lived preferences).`,
		Tags: []string{"resource", "store"},
		Annotations: plugins.ToolAnnotations{
			CostHint: plugins.ToolCostCheap,
			System: `Persist information future turns or sessions may need. Skip transient turn details — those already live in message history.
Choose type carefully: memory for durable facts, reference for cited material, online for fetched pages (add expires_at), global only for truly cross-session data.
Use tags for fast filtering (e.g. ["preference","ui"]). Metadata fields are available as filter predicates in recall.`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"type": {
					Type:        "string",
					Description: "Resource category — determines the storage namespace.",
					Enum:        []string{"memory", "reference", "online", "global"},
				},
				"name": {
					Type:        "string",
					Description: `Short human-readable identifier (e.g. "project-goals", "auth-decision"). If omitted, derived from content hash. Overwriting the same name replaces the previous version and keeps history.`,
				},
				"content": {
					Type:        "string",
					Description: "The text content to store.",
				},
				"tags": {
					Type:        "array",
					Description: `Optional list of string tags (e.g. ["preference", "ui"]). Used for filtering in recall.`,
				},
				"metadata": {
					Type:        "object",
					Description: `Optional structured metadata. For online resources, set expires_at (RFC3339) to indicate freshness. Other keys are available as recall filter predicates.`,
				},
			},
			Required: []string{"type", "content"},
		},
	},
	{
		Name: "recall",
		Description: `Search previously stored resources by type, content query, tags, metadata filters, or time range.

type selects which namespace to search:
  memory    — session memories (facts, preferences, decisions).
  reference — session references (links, docs, specs).
  online    — session web-fetched content.
  global    — agent-wide shared resources.
  any       — all namespaces within the current session (falls back to substring search; HNSW not available).`,
		Tags: []string{"resource", "recall"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
			System: `Search over previously stored resources. Reach for this when the user references prior knowledge ("the thing we decided last week", "my preferences") that is not in the current message history.

filter is a list of {key, op, value} objects (AND semantics):
    op values: eq | prefix | contains | gte | lte
    Example: [{"key":"kind","op":"eq","value":"decision"}]

tags is an AND filter: resource must carry all listed tags. created_after / created_before accept RFC3339 timestamps.
Prefer a specific type over "any" when possible — it enables HNSW semantic search.`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"type": {
					Type:        "string",
					Description: "Namespace to search. Use \"any\" to search across all session namespaces (disables HNSW).",
					Enum:        []string{"memory", "reference", "online", "global", "any"},
				},
				"query": {
					Type:        "string",
					Description: "Content text search. When embed_model is configured and type is not \"any\", uses HNSW semantic search. Empty means no content filter.",
				},
				"tags": {
					Type:        "array",
					Description: `Tag filter (AND): resource must carry all listed tags. Example: ["preference","ui"].`,
				},
				"filter": {
					Type:        "array",
					Description: `Metadata predicates (AND). Each element: {"key": "<field>", "op": "eq|prefix|contains|gte|lte", "value": <any>}.`,
				},
				"created_after": {
					Type:        "string",
					Description: `RFC3339 timestamp. Only return resources created after this time.`,
				},
				"created_before": {
					Type:        "string",
					Description: `RFC3339 timestamp. Only return resources created before this time.`,
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of results to return. Defaults to 5.",
				},
			},
			Required: []string{"type"},
		},
	},
	{
		Name:        "forget",
		Description: `Delete a previously stored resource by type and name.`,
		Tags:        []string{"resource", "forget"},
		Annotations: plugins.ToolAnnotations{
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
			System: `Use when the user asks to remove a stored memory, or when a prior recall surfaced information that is no longer correct.
Pair with recall first to find the right name; do not loop forget over many names without confirmation.`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"type": {
					Type:        "string",
					Description: "Namespace the resource lives in.",
					Enum:        []string{"memory", "reference", "online", "global"},
				},
				"name": {
					Type:        "string",
					Description: "The name (ref key) of the resource to delete.",
				},
			},
			Required: []string{"type", "name"},
		},
	},
}
