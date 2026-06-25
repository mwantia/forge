package tool

import (
	"context"

	plugins "github.com/mwantia/forge-sdk/pkg/plugin"
	"github.com/mwantia/forge-sdk/pkg/plugin/provider"
)

// ToolsRegistar is the narrow surface other services use to register and invoke tools.
type ToolsRegistar interface {
	RegisterTool(namespace string, tool plugins.ToolDefinition, exec ToolsExecution) error

	// RegisterNamespaceMetadata attaches plugin-level metadata (description,
	// version, optional ToolsPlugin handle) to a namespace. Called by the
	// loader once per plugin-driven namespace; built-in namespaces (memory,
	// session) may skip it.
	RegisterNamespaceMetadata(namespace string, meta NamespaceMetadata) error

	ExecuteToolWithCallID(ctx context.Context, namespace, name string, arguments map[string]any, callID string) (*plugins.ExecuteToolResponse, error)

	ExecuteTool(ctx context.Context, namespace, name string, arguments map[string]any) (*plugins.ExecuteToolResponse, error)

	GetToolDefinition(namespace string, name string) (plugins.ToolDefinition, error)

	GetToolDefinitionsByNamespace(namespace string) ([]plugins.ToolDefinition, error)

	GetAllToolDefinitions() ([]plugins.ToolDefinition, error)

	// GetAllToolCalls returns all registered tools as ToolCall values using their
	// fully-qualified "namespace__name" identifier.
	GetAllToolCalls() ([]provider.ToolCall, error)

	// ListNamespaces returns a deterministic snapshot of every registered
	// namespace, sorted ascending by namespace name.
	ListNamespaces() []NamespaceInfo
}
