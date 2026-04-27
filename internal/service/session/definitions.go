package session

import "github.com/mwantia/forge-sdk/pkg/plugins"

// ToolsDefinitions are registered under the "sessions" namespace at Init.
var ToolsDefinitions = []plugins.ToolDefinition{
	{
		Name:        "update_session_title",
		Description: `Set a short, human-readable title for a session that summarises the conversation topic.`,
		Tags:        []string{"session", "update", "metadata"},
		Annotations: plugins.ToolAnnotations{
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
			System: `
Set once the topic is clear (usually after the first 1–2 user turns).
Aim for under 60 characters — this title appears in session lists.
Don't keep retitling on every turn; only update if the topic shifts
substantively.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id": {Type: "string", Description: "The ID of the session to update."},
				"title":      {Type: "string", Description: "The title to assign to this session."},
			},
			Required: []string{"session_id", "title"},
		},
	},
	{
		Name: "update_session_description",
		Description: `Set a longer description for a session that provides additional context about the conversation.
		This description will be used as additional context for each conversation as system prompt.`,
		Tags: []string{"session", "update", "metadata"},
		Annotations: plugins.ToolAnnotations{
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
			System: `
The description is injected as the session-layer system prompt on
every subsequent turn — keep it concise and durable. Use for
constraints/goals that should stick across the whole conversation, not
for one-turn details.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id":  {Type: "string", Description: "The ID of the session to update."},
				"description": {Type: "string", Description: "The description to assign to this session."},
			},
			Required: []string{"session_id", "description"},
		},
	},
	{
		Name:        "read_session",
		Description: `Get the current state of a session by its ID including metadata and session information.`,
		Tags:        []string{"session", "read", "metadata"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
			System: `
Use to inspect another session's metadata (e.g. a sub-session you
spawned). Skip this for the current session — its metadata is already
implicit in the system prompt.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id": {Type: "string", Description: "The ID of the session to retrieve."},
			},
			Required: []string{"session_id"},
		},
	},
	{
		Name:        "list_sub_sessions",
		Description: `List sub-sessions owned by the current session. Optionally filter by a different parent session ID.`,
		Tags:        []string{"session", "list"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
			System: `
Defaults to the current session as the parent. Use when the user asks
"what did we delegate?" or before dispatching to an existing
sub-session. Pagination defaults are sane — only override if the user
asks for older entries.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"parent": {Type: "string", Description: "Filter by parent session ID. Defaults to the current session ID."},
				"limit":  {Type: "integer", Description: "Maximum number of sessions to return. Defaults to 20."},
				"offset": {Type: "integer", Description: "Pagination offset. Defaults to 0."},
			},
		},
	},
	{
		Name:        "create_session",
		Description: `Create a new sub-session owned by the current session. The new session inherits the current session's model and tools unless overridden.`,
		Tags:        []string{"session", "create"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
			System: `
Spawn a sub-session for parallel or sandboxed work — e.g. a research
side-quest that shouldn't pollute the main thread, or a delegated task
with a tighter system prompt. Inherits the current model and tool set
unless overridden. Pair with dispatch_session to actually drive it.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"model":               {Type: "string", Description: "LLM model to use. Defaults to the current session's model."},
				"title":               {Type: "string", Description: "Optional short title for the sub-session."},
				"description":         {Type: "string", Description: "Optional description for the sub-session."},
				"system_prompts":      {Type: "array", Description: "Optional system prompt entries for the sub-session. Each entry has 'name' and 'content' fields."},
				"tools":               {Type: "array", Description: "Tool plugin names to enable. Defaults to the current session's tools."},
				"max_tool_iterations": {Type: "integer", Description: "Maximum tool call iterations. Defaults to the current session's value."},
			},
		},
	},
	{
		Name:        "dispatch_session",
		Description: `Send a message to a session and wait for the full response. Blocks until the sub-session finishes processing.`,
		Tags:        []string{"session", "dispatch"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostModerate,
			System: `
Synchronous: blocks until the target session finishes its full
tool-loop. Use sparingly — every dispatch is a nested LLM run with its
own token cost. Frame the message tightly so the sub-session can
answer and return without further round-trips.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id": {Type: "string", Description: "The ID of the session to dispatch to."},
				"content":    {Type: "string", Description: "The message content to send."},
			},
			Required: []string{"session_id", "content"},
		},
	},
	{
		Name:        "list_message_history",
		Description: `Retrieve the message history of a session.`,
		Tags:        []string{"session", "list", "history", "message"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
			System: `
Use for inspecting a different session's transcript — the current
session's history is already in your context. Pair with read_message
when you need the full body of a specific entry the list returns.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id": {Type: "string", Description: "The ID of the session to list messages from."},
				"limit":      {Type: "integer", Description: "Maximum number of messages to return. Defaults to 50."},
				"offset":     {Type: "integer", Description: "Pagination offset. Defaults to 0."},
			},
			Required: []string{"session_id"},
		},
	},
	{
		Name:        "read_message",
		Description: `Retrieve a single message by its ID from a session.`,
		Tags:        []string{"session", "read", "message"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
			System: `
Use after list_message_history when you've identified one entry worth
reading in full. Don't loop read_message across many IDs to rebuild a
transcript — list_message_history already returns enough metadata.
`,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id": {Type: "string", Description: "The ID of the session containing the message."},
				"message_id": {Type: "string", Description: "The ID of the message to retrieve."},
			},
			Required: []string{"session_id", "message_id"},
		},
	},
}
