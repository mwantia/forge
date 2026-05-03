package resource

import "github.com/mwantia/forge-sdk/pkg/plugins"

// ToolsDefinitions are registered under the "resource" namespace at Init.
// LLM-visible names are prefixed: resource__store, resource__recall,
// resource__forget.
var ToolsDefinitions = []plugins.ToolDefinition{
	{
		Name:        "store",
		Description: `Persist a piece of context (text plus optional tags and metadata) into long-term memory so it can be retrieved by later turns or sessions.`,
		Tags:        []string{"resource", "store"},
		Annotations: plugins.ToolAnnotations{
			CostHint: plugins.ToolCostCheap,
			System: `
Persist information that future turns or future sessions may need:
user preferences, project facts, decisions made, things the user
explicitly asked to remember. Skip transient turn details — those
already live in message history. Use tags to make recall easier
(e.g. ["preference","ui"]). Pass structured metadata when it helps
later filter calls (e.g. {"kind":"decision","topic":"auth"}).
Path defaults to the caller session; pass it explicitly only when
storing shared or cross-session information (e.g. "/global").
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"content": {
					Type:        "string",
					Description: "The text content to store.",
				},
				"path": {
					Type:        "string",
					Description: `Storage path. Exact segment, no wildcards. Examples: "/sessions/abc123", "/global", "/projects/acme". Defaults to the caller session path.`,
				},
				"tags": {
					Type:        "array",
					Description: `Optional list of string tags attached to the resource (e.g. ["preference", "ui"]). Used for filtering in recall.`,
				},
				"metadata": {
					Type:        "object",
					Description: "Optional structured metadata attached to the stored resource. Used in recall filter predicates.",
				},
			},
			Required: []string{"content"},
		},
	},
	{
		Name:        "recall",
		Description: `Search previously stored resources by path, content query, tags, metadata filters, or time range.`,
		Tags:        []string{"resource", "recall"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
			System: `
Search over previously stored resources. Reach for this when the user
references prior knowledge ("the thing we decided last week", "my
preferences") that is not in the current message history.

Path supports glob patterns: * matches a single path segment, ** matches
any number of segments. Examples:
  "/sessions/abc123"       — exact session
  "/sessions/**"           — all sessions
  "/archives/**/consul*"   — any archive whose path segment starts with "consul"

filter is a list of {key, op, value} objects (AND semantics):
  op values: eq | prefix | contains | gte | lte
  Example: [{"key":"kind","op":"eq","value":"decision"}]

tags is an AND filter: resource must carry all listed tags.
created_after / created_before accept RFC3339 timestamps.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"path": {
					Type:        "string",
					Description: `Path or glob to search within. Supports * (single segment) and ** (any depth). Defaults to the caller session path.`,
				},
				"query": {
					Type:        "string",
					Description: "Content text search. Empty means no content filter — all path-matching resources are returned.",
				},
				"tags": {
					Type:        "array",
					Description: `Tag filter (AND): resource must carry all listed tags. Example: ["preference","ui"].`,
				},
				"filter": {
					Type:        "array",
					Description: `Metadata predicates (AND). Each element is an object: {"key": "<field>", "op": "eq|prefix|contains|gte|lte", "value": <any>}. All predicates must match.`,
				},
				"created_after": {
					Type:        "string",
					Description: "RFC3339 timestamp. Only return resources created after this time. Example: \"2026-01-01T00:00:00Z\".",
				},
				"created_before": {
					Type:        "string",
					Description: "RFC3339 timestamp. Only return resources created before this time.",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of results to return. Defaults to 5.",
				},
			},
		},
	},
	{
		Name:        "forget",
		Description: `Delete a previously stored resource by path and ID.`,
		Tags:        []string{"resource", "forget"},
		Annotations: plugins.ToolAnnotations{
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
			System: `
Use when the user asks to remove a stored memory, or when a prior recall
surfaced information that is no longer correct. Pair with recall first
to find the right ID; do not loop forget over many IDs without
confirmation.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"id": {
					Type:        "string",
					Description: "The ID of the resource to delete.",
				},
				"path": {
					Type:        "string",
					Description: "Path where the resource lives. Defaults to the caller session path.",
				},
			},
			Required: []string{"id"},
		},
	},
}
