package sandbox

import "github.com/mwantia/forge-sdk/pkg/plugins"

var toolDefinitions = map[string]plugins.ToolDefinition{
	"sandbox_create": {
		Name:        "sandbox_create",
		Description: "Create a new isolated sandbox for this session. Returns the sandbox ID",
		Tags:        []string{"sandbox", "isolation", "create"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:              false,
			Idempotent:            false,
			IdempotentProbability: plugins.ToolIdempotentGuaranteed,
			CostHint:              plugins.ToolCostCheap,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"name":              {Type: "string", Description: "Human-readable name for the sandbox"},
				"driver":            {Type: "string", Description: "Isolation driver to use (default: \"builtin\")"},
				"working_directory": {Type: "string", Description: "Working directory inside the sandbox for command execution"},
			},
			Required: []string{"driver", "working_directory"},
		},
	},

	"sandbox_destroy": {
		Name:        "sandbox_destroy",
		Description: "Destroy a sandbox and release all its resources",
		Tags:        []string{"sandbox", "isolation", "destroy"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:              false,
			Idempotent:            false,
			IdempotentProbability: plugins.ToolIdempotentGuaranteed,
			CostHint:              plugins.ToolCostCheap,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"id": {Type: "string", Description: "The sandbox ID that should be destroyed"},
			},
			Required: []string{"id"},
		},
	},

	"sandbox_list": {
		Name:        "sandbox_list",
		Description: "List all sandboxes owned by the current session",
		Tags:        []string{"sandbox", "isolation", "list"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:              true,
			Idempotent:            false,
			IdempotentProbability: plugins.ToolIdempotentGuaranteed,
			CostHint:              plugins.ToolCostCheap,
		},
		Parameters: plugins.ToolParameters{
			Type:       "object",
			Properties: make(map[string]plugins.ToolProperty),
		},
	},

	"sandbox_exec": {
		Name:        "sandbox_exec",
		Description: "Execute a command inside a sandbox. Returns combined stdout and stderr output",
		Tags:        []string{"sandbox", "isolation", "exec", "command"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:              false,
			Idempotent:            false,
			IdempotentProbability: plugins.ToolIdempotentGuaranteed,
			CostHint:              plugins.ToolCostExpensive,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"id":      {Type: "string", Description: "The sandbox ID the command will be executed in"},
				"command": {Type: "string", Description: "The absolute path to the command to execute", Format: "/path/to/file"},
				"args":    {Type: "string", Description: "Comma-separated list of command arguments to pass onto"},
				"timeout": {Type: "string", Description: "The execution timeout when the tool call will be forcibly exit (default: \"30s\")"},
			},
			Required: []string{"id", "command"},
		},
	},

	"sandbox_copy_in": {
		Name:        "sandbox_copy_in",
		Description: "Copy a file from the host filesystem into a sandbox",
		Tags:        []string{"sandbox", "isolation", "filesystem", "copy"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:              false,
			Idempotent:            false,
			IdempotentProbability: plugins.ToolIdempotentGuaranteed,
			CostHint:              plugins.ToolCostModerate,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"id":          {Type: "string", Description: "The sandbox ID the file will be copied into"},
				"source":      {Type: "string", Description: "The absolute path on the host to copy from", Format: "/path/to/file"},
				"destination": {Type: "string", Description: "The relative or absolute path inside the sandbox to copy into", Format: "/path/to/file"},
			},
			Required: []string{"id", "source", "destination"},
		},
	},

	"sandbox_copy_out": {
		Name:        "sandbox_copy_out",
		Description: "Copy a file from inside a sandbox to the host filesystem",
		Tags:        []string{"sandbox", "isolation", "filesystem", "copy"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:              false,
			Idempotent:            false,
			IdempotentProbability: plugins.ToolIdempotentGuaranteed,
			CostHint:              plugins.ToolCostModerate,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"id":          {Type: "string", Description: "The sandbox ID the file will be copied from"},
				"source":      {Type: "string", Description: "The relative or absolute path inside the sandbox to copy from", Format: "/path/to/file"},
				"destination": {Type: "string", Description: "The absolute path on the host to copy into", Format: "/path/to/file"},
			},
			Required: []string{"id", "source", "destination"},
		},
	},

	"sandbox_stat": {
		Name:        "sandbox_stat",
		Description: "Check whether a path exists inside a sandbox and return basic file info",
		Tags:        []string{"sandbox", "isolation", "stat", "exists"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:              true,
			Idempotent:            false,
			IdempotentProbability: plugins.ToolIdempotentGuaranteed,
			CostHint:              plugins.ToolCostCheap,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"id":   {Type: "string", Description: "The sandbox ID to use"},
				"path": {Type: "string", Description: "The path inside the sandbox to stat", Format: "/path/to/file"},
			},
			Required: []string{"id", "path"},
		},
	},

	"sandbox_read": {
		Name:        "sandbox_read",
		Description: "Read the content of a file from inside a sandbox",
		Tags:        []string{"sandbox", "isolation", "read"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:              true,
			Idempotent:            false,
			IdempotentProbability: plugins.ToolIdempotentGuaranteed,
			CostHint:              plugins.ToolCostCheap,
		},
		Parameters: plugins.ToolParameters{
			Type: "object",
			Properties: map[string]plugins.ToolProperty{
				"id":   {Type: "string", Description: "The sandbox ID to use"},
				"path": {Type: "string", Description: "The path inside the sandbox to read from", Format: "/path/to/file"},
			},
			Required: []string{"id", "path"},
		},
	},
}
