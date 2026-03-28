package nomad

import "github.com/mwantia/forge/pkg/plugins"

var toolDefinitions = map[string]plugins.ToolDefinition{
	"jobs_list": {
		Name:        "jobs_list",
		Description: "List all jobs registered in the Nomad cluster",
		Tags:        []string{"nomad", "jobs"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{
					"type":        "string",
					"description": "Namespace to query (defaults to driver namespace)",
				},
				"prefix": map[string]any{
					"type":        "string",
					"description": "Filter jobs by ID prefix",
				},
			},
		},
	},

	"job_get": {
		Name:        "job_get",
		Description: "Get the full specification and status of a specific Nomad job",
		Tags:        []string{"nomad", "jobs"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"job_id": map[string]any{
					"type":        "string",
					"description": "Job ID to retrieve",
				},
				"namespace": map[string]any{
					"type":        "string",
					"description": "Namespace the job belongs to",
				},
			},
			"required": []string{"job_id"},
		},
	},

	"job_summary": {
		Name:        "job_summary",
		Description: "Get a summary of task group statuses for a Nomad job",
		Tags:        []string{"nomad", "jobs"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"job_id": map[string]any{
					"type":        "string",
					"description": "Job ID to summarise",
				},
				"namespace": map[string]any{
					"type":        "string",
					"description": "Namespace the job belongs to",
				},
			},
			"required": []string{"job_id"},
		},
	},

	"job_submit": {
		Name:        "job_submit",
		Description: "Register or update a Nomad job from a JSON job specification",
		Tags:        []string{"nomad", "jobs"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:    false,
			Destructive: false,
			Idempotent:  true,
			CostHint:    "moderate",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"job_spec": map[string]any{
					"type":        "string",
					"description": "JSON-encoded Nomad job specification (api.Job struct)",
				},
			},
			"required": []string{"job_spec"},
		},
	},

	"job_stop": {
		Name:        "job_stop",
		Description: "Stop (deregister) a running Nomad job",
		Tags:        []string{"nomad", "jobs"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:             false,
			Destructive:          true,
			Idempotent:           false,
			RequiresConfirmation: true,
			CostHint:             "moderate",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"job_id": map[string]any{
					"type":        "string",
					"description": "Job ID to stop",
				},
				"purge": map[string]any{
					"type":        "boolean",
					"description": "Permanently remove the job from the state store",
				},
				"namespace": map[string]any{
					"type":        "string",
					"description": "Namespace the job belongs to",
				},
			},
			"required": []string{"job_id"},
		},
	},

	"allocations_list": {
		Name:        "allocations_list",
		Description: "List allocations in the Nomad cluster, optionally filtered by job",
		Tags:        []string{"nomad", "allocations"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"job_id": map[string]any{
					"type":        "string",
					"description": "Filter allocations to this job ID",
				},
				"namespace": map[string]any{
					"type":        "string",
					"description": "Namespace to query",
				},
			},
		},
	},

	"allocation_get": {
		Name:        "allocation_get",
		Description: "Get details of a specific Nomad allocation including task states",
		Tags:        []string{"nomad", "allocations"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"alloc_id": map[string]any{
					"type":        "string",
					"description": "Allocation ID (full or prefix)",
				},
			},
			"required": []string{"alloc_id"},
		},
	},

	"nodes_list": {
		Name:        "nodes_list",
		Description: "List all client nodes registered in the Nomad cluster",
		Tags:        []string{"nomad", "nodes"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prefix": map[string]any{
					"type":        "string",
					"description": "Filter nodes by ID prefix",
				},
			},
		},
	},

	"node_get": {
		Name:        "node_get",
		Description: "Get details of a specific Nomad client node including resources and attributes",
		Tags:        []string{"nomad", "nodes"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"node_id": map[string]any{
					"type":        "string",
					"description": "Node ID to retrieve",
				},
			},
			"required": []string{"node_id"},
		},
	},

	"evaluations_list": {
		Name:        "evaluations_list",
		Description: "List scheduler evaluations in the Nomad cluster",
		Tags:        []string{"nomad", "evaluations"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"job_id": map[string]any{
					"type":        "string",
					"description": "Filter evaluations by job ID",
				},
				"namespace": map[string]any{
					"type":        "string",
					"description": "Namespace to query",
				},
			},
		},
	},

	"namespaces_list": {
		Name:        "namespaces_list",
		Description: "List all namespaces in the Nomad cluster",
		Tags:        []string{"nomad", "namespaces"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},

	"agent_members": {
		Name:        "agent_members",
		Description: "List all server members visible to the Nomad agent",
		Tags:        []string{"nomad", "agent"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},

	"agent_self": {
		Name:        "agent_self",
		Description: "Get configuration and stats of the local Nomad agent",
		Tags:        []string{"nomad", "agent"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   "cheap",
		},
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
}
