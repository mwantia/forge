package session

import "github.com/mwantia/forge-sdk/pkg/plugins"

var toolDefinitions = map[string]plugins.ToolDefinition{
	"session_set_title": {
		Name:        "session_set_title",
		Description: "Set a short, human-readable title for the current session that summarises the conversation topic.",
		Tags:        []string{"session"},
		Annotations: plugins.ToolAnnotations{
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"title": {Type: "string", Description: "The title to assign to this session."},
			},
			Required: []string{"title"},
		},
	},
	"session_set_description": {
		Name:        "session_set_description",
		Description: "Set a longer description for the current session that provides additional context about the conversation.",
		Tags:        []string{"session"},
		Annotations: plugins.ToolAnnotations{
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"description": {Type: "string", Description: "The description to assign to this session."},
			},
			Required: []string{"description"},
		},
	},
	"session_list_sessions": {
		Name:        "session_list_sessions",
		Description: "List sub-sessions owned by the current session. Optionally filter by a different parent session ID.",
		Tags:        []string{"session"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
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
	"session_get_session": {
		Name:        "session_get_session",
		Description: "Get the current state (metadata) of a session by its ID.",
		Tags:        []string{"session"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id": {Type: "string", Description: "The ID of the session to retrieve."},
			},
			Required: []string{"session_id"},
		},
	},
	"session_create_session": {
		Name:        "session_create_session",
		Description: "Create a new sub-session owned by the current session. The new session inherits the current session's model and tools unless overridden.",
		Tags:        []string{"session"},
		Annotations: plugins.ToolAnnotations{
			CostHint: plugins.ToolCostCheap,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"model":               {Type: "string", Description: "LLM model to use. Defaults to the current session's model."},
				"title":               {Type: "string", Description: "Optional short title for the sub-session."},
				"description":         {Type: "string", Description: "Optional description for the sub-session."},
				"system_prompt":       {Type: "string", Description: "Optional system prompt for the sub-session."},
				"tools":               {Type: "array", Description: "Tool plugin names to enable. Defaults to the current session's tools."},
				"max_tool_iterations": {Type: "integer", Description: "Maximum tool call iterations. Defaults to the current session's value."},
			},
		},
	},
	"session_dispatch_session": {
		Name:        "session_dispatch_session",
		Description: "Send a message to a sub-session and wait for the full response. Blocks until the sub-session finishes processing.",
		Tags:        []string{"session"},
		Annotations: plugins.ToolAnnotations{
			CostHint: plugins.ToolCostModerate,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id": {Type: "string", Description: "The ID of the sub-session to dispatch to."},
				"content":    {Type: "string", Description: "The message content to send."},
			},
			Required: []string{"session_id", "content"},
		},
	},
	"session_get_history": {
		Name:        "session_get_history",
		Description: "Retrieve the message history of a session.",
		Tags:        []string{"session"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"session_id": {Type: "string", Description: "The ID of the session."},
				"limit":      {Type: "integer", Description: "Maximum number of messages to return. Defaults to 50."},
				"offset":     {Type: "integer", Description: "Pagination offset. Defaults to 0."},
			},
			Required: []string{"session_id"},
		},
	},
	"session_get_message": {
		Name:        "session_get_message",
		Description: "Retrieve a single message by its ID from a session.",
		Tags:        []string{"session"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostFree,
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
