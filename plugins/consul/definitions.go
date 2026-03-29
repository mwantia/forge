package consul

import "github.com/mwantia/forge/pkg/plugins"

var toolDefinitions = map[string]plugins.ToolDefinition{
	"catalog_datacenters": {
		Name:        "catalog_datacenters",
		Description: "List all known datacenters in the Consul cluster",
		Tags:        []string{"consul", "catalog"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},

	"catalog_nodes": {
		Name:        "catalog_nodes",
		Description: "List all nodes registered in the Consul catalog",
		Tags:        []string{"consul", "catalog", "nodes"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"datacenter": map[string]any{
					"type":        "string",
					"description": "Datacenter to query (defaults to agent's datacenter)",
				},
				"filter": map[string]any{
					"type":        "string",
					"description": "Consul filter expression (e.g. 'Meta.env == \"production\"')",
				},
			},
		},
	},

	"catalog_services": {
		Name:        "catalog_services",
		Description: "List all services registered in the Consul catalog with their tags",
		Tags:        []string{"consul", "catalog", "services"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"datacenter": map[string]any{
					"type":        "string",
					"description": "Datacenter to query (defaults to agent's datacenter)",
				},
			},
		},
	},

	"catalog_service": {
		Name:        "catalog_service",
		Description: "Get all nodes providing a specific service, including health status and metadata",
		Tags:        []string{"consul", "catalog", "services"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service name to look up",
				},
				"tag": map[string]any{
					"type":        "string",
					"description": "Filter results to nodes with this tag",
				},
				"datacenter": map[string]any{
					"type":        "string",
					"description": "Datacenter to query",
				},
			},
			"required": []any{"service"},
		},
	},

	"catalog_node": {
		Name:        "catalog_node",
		Description: "Get all services registered on a specific node",
		Tags:        []string{"consul", "catalog", "nodes"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"node": map[string]any{
					"type":        "string",
					"description": "Node name or ID",
				},
				"datacenter": map[string]any{
					"type":        "string",
					"description": "Datacenter to query",
				},
			},
			"required": []any{"node"},
		},
	},

	"health_service": {
		Name:        "health_service",
		Description: "Get health status of all instances of a service",
		Tags:        []string{"consul", "health", "services"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service name",
				},
				"tag": map[string]any{
					"type":        "string",
					"description": "Filter by tag",
				},
				"passing_only": map[string]any{
					"type":        "boolean",
					"description": "Return only instances passing all health checks",
				},
				"datacenter": map[string]any{
					"type":        "string",
					"description": "Datacenter to query",
				},
			},
			"required": []any{"service"},
		},
	},

	"health_node": {
		Name:        "health_node",
		Description: "Get all health checks for a specific node",
		Tags:        []string{"consul", "health", "nodes"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"node": map[string]any{
					"type":        "string",
					"description": "Node name",
				},
				"datacenter": map[string]any{
					"type":        "string",
					"description": "Datacenter to query",
				},
			},
			"required": []any{"node"},
		},
	},

	"health_checks": {
		Name:        "health_checks",
		Description: "List health checks filtered by state (passing, warning, critical, any)",
		Tags:        []string{"consul", "health"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"state": map[string]any{
					"type":        "string",
					"description": "Health check state: 'passing', 'warning', 'critical', or 'any'",
					"enum":        []any{"passing", "warning", "critical", "any"},
				},
				"datacenter": map[string]any{
					"type":        "string",
					"description": "Datacenter to query",
				},
			},
			"required": []any{"state"},
		},
	},

	"kv_get": {
		Name:        "kv_get",
		Description: "Read a value from the Consul key-value store",
		Tags:        []string{"consul", "kv"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "Key path to read",
				},
				"datacenter": map[string]any{
					"type":        "string",
					"description": "Datacenter to query",
				},
			},
			"required": []any{"key"},
		},
	},

	"kv_list": {
		Name:        "kv_list",
		Description: "List all keys in the Consul KV store under a prefix",
		Tags:        []string{"consul", "kv"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prefix": map[string]any{
					"type":        "string",
					"description": "Key prefix to list (use empty string for all keys)",
				},
				"datacenter": map[string]any{
					"type":        "string",
					"description": "Datacenter to query",
				},
			},
			"required": []any{"prefix"},
		},
	},

	"kv_put": {
		Name:        "kv_put",
		Description: "Write a value to the Consul key-value store",
		Tags:        []string{"consul", "kv"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:    false,
			Destructive: false,
			Idempotent:  true,
			CostHint:    plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "Key path to write",
				},
				"value": map[string]any{
					"type":        "string",
					"description": "Value to store (string)",
				},
				"datacenter": map[string]any{
					"type":        "string",
					"description": "Datacenter to write to",
				},
			},
			"required": []any{"key", "value"},
		},
	},

	"kv_delete": {
		Name:        "kv_delete",
		Description: "Delete a key or key prefix from the Consul key-value store",
		Tags:        []string{"consul", "kv"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:             false,
			Destructive:          true,
			Idempotent:           true,
			RequiresConfirmation: true,
			CostHint:             plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key": map[string]any{
					"type":        "string",
					"description": "Key path to delete",
				},
				"recurse": map[string]any{
					"type":        "boolean",
					"description": "Delete all keys with this prefix (recursive delete)",
				},
				"datacenter": map[string]any{
					"type":        "string",
					"description": "Datacenter to target",
				},
			},
			"required": []any{"key"},
		},
	},

	"agent_members": {
		Name:        "agent_members",
		Description: "List all members visible to the local Consul agent (Serf gossip members)",
		Tags:        []string{"consul", "agent"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"wan": map[string]any{
					"type":        "boolean",
					"description": "List WAN members instead of LAN members",
				},
			},
		},
	},

	"agent_services": {
		Name:        "agent_services",
		Description: "List all services registered with the local Consul agent",
		Tags:        []string{"consul", "agent", "services"},
		Annotations: plugins.ToolAnnotations{
			ReadOnly:   true,
			Idempotent: true,
			CostHint:   plugins.ToolCostCheap,
		},
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
}
