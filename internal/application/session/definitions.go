package session

import plugins "github.com/mwantia/forge-sdk/pkg/plugin"

// ToolsDefinitions are registered under the "builtin" namespace at PostInit.
var ToolsDefinitions = []plugins.ToolDefinition{
	{
		Name:        "update_session",
		Description: `Update the title and/or description of a session. Provide one or both fields; only supplied fields are changed.`,
		Tags:        []string{"session", "update", "metadata"},
		Annotations: plugins.ToolAnnotations{
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
			System: `Set title once the topic is clear (usually after the first 1–2 user turns). Aim for under 60 characters.
Set description for constraints or goals that should persist across the whole conversation — it is injected as session context on every subsequent turn.
Don't keep re-updating on every turn; only update if the topic or goals shift substantively.
Execute as soon as the direction of a session is clear and no title/description have been set or declared for this session yet.`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id":  {Type: "string", Description: "The ID of the session to update. Defaults to the caller session."},
				"title":       {Type: "string", Description: "Short human-readable title (under 60 characters)."},
				"description": {Type: "string", Description: "Longer description injected as session-layer context on every turn."},
				"mode":        {Type: "string", Description: `Active routing mode. Canonical values: "chat" (default), "plan", "code", "research". Any non-empty string is accepted for custom routing. Set to "chat" to clear specialization.`, Enum: []string{"chat", "plan", "code", "research"}},
			},
		},
	},
	{
		Name:        "query_sessions",
		Description: `List siblings. Optionally filter by parent, archived status, and paginate.`,
		Tags:        []string{"session", "list"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
			System: `Defaults to the current session as parent. Use when the user asks "what did we delegate?" or before committing to an existing sibling.
Pass archived=true to list archived sessions, archived=false (default) for active ones.
Pagination defaults are sane — only override if the user asks for older entries.`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"parent":   {Type: "string", Description: "Filter by parent session ID. Defaults to the current session."},
				"archived": {Type: "boolean", Description: "true = archived only, false = active only (default). Omit to return both."},
				"limit":    {Type: "integer", Description: "Maximum number of sessions to return. Defaults to 20."},
				"offset":   {Type: "integer", Description: "Pagination offset. Defaults to 0."},
			},
		},
	},
	{
		Name:        "create_session",
		Description: `Create a new sibling owned by the current session.`,
		Tags:        []string{"session", "create"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
			System: `Spawn a focused sibling for a specific delegated task. Always scope to only the plugins it actually needs:

- Set "plugins" to the minimal list of plugin namespaces required (e.g. ["skills"] for a file task, ["consul"] for a service-discovery lookup).
  The builtin namespace is always available regardless of this list.

Narrowing plugins keeps the context window small and the model focused. Pair with builtin__commit_session to drive the sibling.`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"model":       {Type: "string", Description: "LLM model to use. Defaults to the current session's model."},
				"title":       {Type: "string", Description: "Optional short title for the sibling."},
				"description": {Type: "string", Description: "Optional description for the sibling."},
				"system":      {Type: "string", Description: "Optional system prompt for the sibling."},
				"plugins":     {Type: "array", Description: `Plugin namespaces to allow (e.g. ["skills", "consul"]). The builtin namespace is always active.`},
			},
		},
	},
	{
		Name:        "query_messages",
		Description: `Retrieve the message history of a session with optional filtering.`,
		Tags:        []string{"session", "list", "history", "message"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
			System: `Use for inspecting a different session's transcript — the current session's history is already in your context.
Filter by role ("user", "assistant", "tool") or has_tool_calls to narrow results before paginating.`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id":    {Type: "string", Description: "The ID of the session to list messages from."},
				"role":          {Type: "string", Description: `Optional role filter: "user", "assistant", or "tool".`},
				"has_tool_calls": {Type: "boolean", Description: "When true, return only messages that contain tool calls."},
				"limit":         {Type: "integer", Description: "Maximum number of messages to return. Defaults to 50."},
				"offset":        {Type: "integer", Description: "Pagination offset. Defaults to 0."},
			},
		},
	},
	{
		Name:        "archive_session",
		Description: `Walk a session ref to root, persist an archive envelope through the resource layer, and flip the session to immutable.`,
		Tags:        []string{"session", "archive"},
		Annotations: plugins.ToolAnnotations{
			CostHint: plugins.ToolCostModerate,
			System: `Archive a session when the user signals it's done — wrap-up phrasing, "save this for later", or before a long pause. The session becomes immutable: no further commits or ref moves succeed.
Pair with builtin__clone_session to fork a live successor off the archive.`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id": {Type: "string", Description: "The ID of the session to archive. Defaults to the caller session."},
				"ref":        {Type: "string", Description: "Ref to archive. Defaults to HEAD."},
			},
		},
	},
	{
		Name:        "clone_session",
		Description: `Replay a session archive into a fresh live session whose HEAD points at the archived tip.`,
		Tags:        []string{"session", "clone"},
		Annotations: plugins.ToolAnnotations{
			CostHint: plugins.ToolCostModerate,
			System: `Use to revive an archived session as a new live conversation — e.g. the user wants to "pick up where we left off".
The clone has its own ID and ref set; lineage to the source is recorded as parent.`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"source_id": {Type: "string", Description: "Source session ID (archived session) or archive resource ID."},
				"name":      {Type: "string", Description: "Optional name for the clone. Defaults to <source-name>-clone-<n>."},
			},
		},
	},
}
