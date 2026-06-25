package tool

import (
	"context"

	"github.com/mwantia/forge-sdk/pkg/plugin/provider"
	"github.com/mwantia/forge-sdk/pkg/plugin/tool"
)

// ToolsRegistar is the narrow surface other services use to register and invoke tools.
type ToolsRegistar interface {
	RegisterTool(namespace string, tool tool.ToolDefinition, exec ToolsExecution) error

	// RegisterNamespaceMetadata attaches plugin-level metadata (description,
	// version, optional ToolsPlugin handle) to a namespace. Called by the
	// loader once per plugin-driven namespace; built-in namespaces (memory,
	// session) may skip it.
	RegisterNamespaceMetadata(namespace string, meta NamespaceMetadata) error

	ExecuteToolWithCallID(ctx context.Context, namespace, name string, arguments map[string]any, callID string) (*tool.ExecuteToolResponse, error)

	ExecuteTool(ctx context.Context, namespace, name string, arguments map[string]any) (*tool.ExecuteToolResponse, error)

	GetToolDefinition(namespace string, name string) (tool.ToolDefinition, error)

	GetToolDefinitionsByNamespace(namespace string) ([]tool.ToolDefinition, error)

	GetAllToolDefinitions() ([]tool.ToolDefinition, error)

	// GetAllToolCalls returns all registered tools as ToolCall values using their
	// fully-qualified "namespace__name" identifier.
	GetAllToolCalls() ([]provider.ToolCall, error)

	// ListNamespaces returns a deterministic snapshot of every registered
	// namespace, sorted ascending by namespace name.
	ListNamespaces() []NamespaceInfo
}
