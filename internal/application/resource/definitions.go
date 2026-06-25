package resource

import "github.com/mwantia/forge-sdk/pkg/plugin/tool"

// ToolsDefinitions are registered under the "resource" namespace at Init.
// LLM-visible names are prefixed: resource__store, resource__recall,
// resource__forget.
var ToolsDefinitions = []tool.ToolDefinition{
	{
		Name:        "store",
		Description: `Persist a piece of context into long-term memory for retrieval across turns or sessions.`,
		Tags:        []string{"resource", "store"},
		Annotations: tool.ToolAnnotations{
			CostHint: tool.ToolCostCheap,
			System: `Persist information future turns or sessions may need. Skip transient turn details — those already live in message history.

type values:
  memory    — facts, preferences, decisions, recurring constraints.
  reference — external links, cited documents, API specs, code snippets.
  online    — web-fetched or time-sensitive content. Set extra.expires_at (RFC3339) to mark freshness.
  archive   — reserved for session archives; do not use directly.

Resources are scoped to the current session automatically. Use recall with scope="global" to search across sessions.
Use tags for fast filtering (e.g. ["preference","ui"]). Named resources (name field) can be updated by storing again with the same name — name is a label, not a unique key.`,
		},
		Parameters: tool.ToolParameters{
			Type: "object",
			Properties: map[string]tool.ToolProperty{
				"content": {
					Type:        "string",
					Description: "The text content to store.",
				},
				"name": {
					Type:        "string",
					Description: `Short human-readable label (e.g. "project-goals", "auth-decision"). Optional; used for display and recall filtering only.`,
				},
				"type": {
					Type:        "string",
					Description: "Resource category.",
					Enum:        []string{"memory", "reference", "online"},
				},
				"description": {
					Type:        "string",
					Description: "Optional longer description of what this resource contains.",
				},
				"tags": {
					Type:        "array",
					Description: `Optional list of string tags (e.g. ["preference", "ui"]). Used for filtering in recall.`,
				},
				"commit_message": {
					Type:        "string",
					Description: `Optional short message describing what this resource is or why it was created (e.g. "initial save", "project goals as of kick-off").`,
				},
				"extra": {
					Type:        "object",
					Description: `Optional unstructured metadata. For online resources, set expires_at (RFC3339) to indicate freshness.`,
				},
			},
			Required: []string{"content"},
		},
	},
	{
		Name:        "commit",
		Description: `Update the content of an existing resource, creating a new versioned revision and advancing HEAD.`,
		Tags:        []string{"resource", "commit"},
		Annotations: tool.ToolAnnotations{
			CostHint: tool.ToolCostCheap,
			System: `Use when the user asks to update, revise, or overwrite an existing stored resource. Requires the resource ID from a prior store or recall result.

The previous content is preserved in the revision chain and can be retrieved via history, diff, or revert. HEAD always points to the latest revision.
Provide a short commit_message so the history list is human-readable (e.g. "updated auth decision after team review").`,
		},
		Parameters: tool.ToolParameters{
			Type: "object",
			Properties: map[string]tool.ToolProperty{
				"id": {
					Type:        "string",
					Description: "The ID of the resource to update (returned by store or visible in recall results).",
				},
				"content": {
					Type:        "string",
					Description: "The new full content to store as the next revision.",
				},
				"commit_message": {
					Type:        "string",
					Description: `Short description of what changed and why (e.g. "revised project goals after retro").`,
				},
			},
			Required: []string{"id", "content"},
		},
	},
	{
		Name: "recall",
		Description: `Search previously stored resources by content query, type, tags, metadata filters, or time range.

scope controls session scoping:
  session   — only resources from the current session (default for most queries).
  global    — all resources regardless of session.

Use type to filter by resource category. Combine with query for semantic or substring search.`,
		Tags: []string{"resource", "recall"},
		Annotations: tool.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   tool.ToolCostCheap,
			System: `Search over previously stored resources. Reach for this when the user references prior knowledge ("the thing we decided last week", "my preferences") that is not in the current message history.

filter is a list of {key, op, value} objects (AND semantics). Valid keys: name, type, session, description, or any key in extra.
    op values: eq | prefix | contains | gte | lte
    Example: [{"key":"type","op":"eq","value":"reference"}]

tags is an AND filter: resource must carry all listed tags. created_after / created_before accept RFC3339 timestamps.
Prefer scope="session" unless the user explicitly asks for global results.`,
		},
		Parameters: tool.ToolParameters{
			Type: "object",
			Properties: map[string]tool.ToolProperty{
				"query": {
					Type:        "string",
					Description: "Content text search. Uses semantic search when embed_model is configured, otherwise substring. Empty means no content filter.",
				},
				"type": {
					Type:        "string",
					Description: "Filter by resource type (eq match).",
					Enum:        []string{"memory", "reference", "online"},
				},
				"scope": {
					Type:        "string",
					Description: `"session" to restrict to the current session (recommended); "global" to search all sessions.`,
					Enum:        []string{"session", "global"},
				},
				"tags": {
					Type:        "array",
					Description: `Tag filter (AND): resource must carry all listed tags.`,
				},
				"filter": {
					Type:        "array",
					Description: `Metadata predicates (AND). Each element: {"key": "<field>", "op": "eq|prefix|contains|gte|lte", "value": <any>}. Valid keys: name, type, session, description, or any extra key.`,
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
		},
	},
	{
		Name:        "forget",
		Description: `Delete a previously stored resource by its ID.`,
		Tags:        []string{"resource", "forget"},
		Annotations: tool.ToolAnnotations{
			Idempotent: true,
			CostHint:   tool.ToolCostCheap,
			System: `Use when the user asks to remove a stored memory, or when a prior recall surfaced information that is no longer correct.
Pair with recall first to find the right ID; do not loop forget over many IDs without confirmation.`,
		},
		Parameters: tool.ToolParameters{
			Type: "object",
			Properties: map[string]tool.ToolProperty{
				"id": {
					Type:        "string",
					Description: "The unique ID of the resource to delete (returned by store or visible in recall results).",
				},
			},
			Required: []string{"id"},
		},
	},
}
