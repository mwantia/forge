package filesystem

import "github.com/mwantia/forge/pkg/plugins"

// toolDefinitions maps tool names to their JSON Schema definitions.
var toolDefinitions = map[string]plugins.ToolDefinition{
	"create": {
		Name:        "create",
		Description: "Create a new file with optional content",
		Tags:        []string{"filesystem", "write"},
		Annotations: plugins.ToolAnnotations{
			Idempotent: false,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path relative to workspace home",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Initial file content (optional, defaults to empty)",
				},
			},
			"required": []string{"path"},
		},
	},
	"read": {
		Name:        "read",
		Description: "Read the contents of a file",
		Tags:        []string{"filesystem", "read"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path relative to workspace home",
				},
			},
			"required": []string{"path"},
		},
	},
	"write": {
		Name:        "write",
		Description: "Write content to a file, creating it if it does not exist",
		Tags:        []string{"filesystem", "write"},
		Annotations: plugins.ToolAnnotations{
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path relative to workspace home",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			"required": []string{"path", "content"},
		},
	},
	"delete": {
		Name:        "delete",
		Description: "Delete a file or directory (recursive)",
		Tags:        []string{"filesystem", "write"},
		Annotations: plugins.ToolAnnotations{
			Destructive:          true,
			RequiresConfirmation: true,
			CostHint:             "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File or directory path relative to workspace home",
				},
			},
			"required": []string{"path"},
		},
	},
	"list": {
		Name:        "list",
		Description: "List the contents of a directory",
		Tags:        []string{"filesystem", "read"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "free",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path relative to workspace home (defaults to home directory)",
				},
			},
		},
	},
	"exec": {
		Name:        "exec",
		Description: "Execute a command in the workspace directory",
		Tags:        []string{"system", "exec"},
		Annotations: plugins.ToolAnnotations{
			RequiresConfirmation: true,
			CostHint:             "expensive",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Command to execute",
				},
				"args": map[string]any{
					"type":        "array",
					"description": "Arguments to pass to the command",
					"items":       map[string]any{"type": "string"},
				},
				"workdir": map[string]any{
					"type":        "string",
					"description": "Working directory for the command (defaults to workspace home)",
				},
			},
			"required": []string{"command"},
		},
	},
}
