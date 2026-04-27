package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/metrics"
)

type ToolsRegistar interface {
	RegisterTool(namespace string, tool plugins.ToolDefinition, exec ToolsExecution) error

	// RegisterNamespaceMetadata attaches plugin-level metadata (description,
	// version, optional ToolsPlugin handle) to a namespace. Called by the
	// loader once per plugin-driven namespace; built-in namespaces (memory,
	// session) may skip it.
	RegisterNamespaceMetadata(namespace string, meta NamespaceMetadata) error

	ExecuteToolWithCallID(ctx context.Context, namespace, name string, arguments map[string]any, callID string) (*plugins.ExecuteResponse, error)

	ExecuteTool(ctx context.Context, namespace, name string, arguments map[string]any) (*plugins.ExecuteResponse, error)

	GetToolDefinition(namespace string, name string) (plugins.ToolDefinition, error)

	GetToolDefinitionsByNamespace(namespace string) ([]plugins.ToolDefinition, error)

	GetAllToolDefinitions() ([]plugins.ToolDefinition, error)

	// GetAllToolCalls returns all registered tools as ToolCall values using their
	// fully-qualified "namespace__name" identifier. Use this when building the tool
	// list to pass to an LLM provider — the pipeline strips the prefix on execution.
	GetAllToolCalls() ([]plugins.ToolCall, error)

	// ListNamespaces returns a deterministic snapshot of every registered
	// namespace, sorted ascending by namespace name. Tools within each
	// namespace are sorted ascending by their fully-qualified name. Use this
	// to assemble the system prompt — alphabetic ordering keeps the prefix
	// byte-stable across pipeline turns for cache reuse.
	ListNamespaces() []NamespaceInfo
}

// NamespaceMetadata is the per-namespace data captured at plugin load time.
// Plugin handle is optional; namespaces registered by built-in services without
// a backing ToolsPlugin (e.g. session bookkeeping) leave it nil and may
// provide a static System prompt instead.
type NamespaceMetadata struct {
	Description string
	Version     string
	Plugin      plugins.ToolsPlugin
	// Builtin marks namespaces owned by the agent process itself (memory,
	// sessions). Builtins always sit in a fixed cache-stable section of the
	// assembled system prompt, ahead of dynamically loaded plugins.
	Builtin bool
	// System is a static plugin-level system prompt fragment. Only consulted
	// when Plugin is nil — plugin-backed namespaces source their fragment
	// from ToolsPlugin.System(ctx) at request time.
	System string
}

// NamespaceInfo is the public, sorted view returned by ListNamespaces.
type NamespaceInfo struct {
	Namespace   string
	Description string
	Version     string
	Plugin      plugins.ToolsPlugin
	Builtin     bool
	System      string
	Tools       []plugins.ToolDefinition
}

type ToolsExecution = func(ctx context.Context, request plugins.ExecuteRequest) (*plugins.ExecuteResponse, error)

func (s *ToolsService) getToolNamespace(namespace, name string) (*ToolsNamespace, bool) {
	ns := strings.ToLower(namespace)
	tools, ok := s.namespaces[ns]
	if !ok {
		return nil, false
	}
	target := strings.ToLower(name)
	fullTarget := ns + "__" + strings.TrimPrefix(target, ns+"__")
	for _, tool := range tools {
		if strings.EqualFold(fullTarget, tool.FullName) {
			return tool, true
		}
	}
	return nil, false
}

func (s *ToolsService) RegisterTool(namespace string, definition plugins.ToolDefinition, exec ToolsExecution) error {
	key := strings.ToLower(namespace)
	bare := strings.ToLower(definition.Name)
	if strings.Contains(bare, "__") {
		return fmt.Errorf("tool name %q must not contain %q; the namespace %q is prepended by the registry", definition.Name, "__", namespace)
	}
	name := key + "__" + bare
	definition.Name = name

	s.namespaces[key] = append(s.namespaces[key], &ToolsNamespace{
		FullName:       name,
		ToolDefinition: definition,
		Execution:      exec,
	})

	ToolsTotal.WithLabelValues(key).Inc()

	return nil
}

func (s *ToolsService) ExecuteToolWithCallID(ctx context.Context, namespace, name string, arguments map[string]any, callID string) (*plugins.ExecuteResponse, error) {
	tool, ok := s.getToolNamespace(namespace, name)
	if !ok {
		return nil, fmt.Errorf("tool with namespace %q and name %q not found", namespace, name)
	}

	start := time.Now()
	resp, err := tool.Execution(ctx, plugins.ExecuteRequest{
		Tool:      tool.ToolDefinition.Name,
		Arguments: arguments,
		CallID:    callID,
	})

	ToolsExecutionDuration.WithLabelValues(namespace, name).Observe(time.Since(start).Seconds())
	status := metrics.ErrToStatusLabel(err)
	if err == nil && resp != nil && resp.IsError {
		status = "error"
	}
	ToolsExecutionsTotal.WithLabelValues(namespace, name, status).Inc()

	return resp, err
}

func (s *ToolsService) ExecuteTool(ctx context.Context, namespace, name string, arguments map[string]any) (*plugins.ExecuteResponse, error) {
	return s.ExecuteToolWithCallID(ctx, namespace, name, arguments, "")
}

func (s *ToolsService) GetToolDefinition(namespace, name string) (plugins.ToolDefinition, error) {
	tool, ok := s.getToolNamespace(namespace, name)
	if ok {
		return tool.ToolDefinition, nil
	}

	return plugins.ToolDefinition{}, fmt.Errorf("tool definition with namespace %q and name %q not found", namespace, name)
}

func (s *ToolsService) GetToolDefinitionsByNamespace(namespace string) ([]plugins.ToolDefinition, error) {
	tools, ok := s.namespaces[strings.ToLower(namespace)]
	if ok {
		definitions := make([]plugins.ToolDefinition, 0, len(tools))
		for _, tool := range tools {
			definitions = append(definitions, tool.ToolDefinition)
		}

		return definitions, nil
	}

	return nil, fmt.Errorf("no tools with namespace %q found", namespace)
}

func (s *ToolsService) GetAllToolDefinitions() ([]plugins.ToolDefinition, error) {
	definitions := make([]plugins.ToolDefinition, 0)
	for _, namespace := range s.namespaces {
		for _, tool := range namespace {
			definitions = append(definitions, tool.ToolDefinition)
		}
	}

	return definitions, nil
}

func (*ToolsService) SplitToolCallName(s string) (string, string, bool) {
	parts := strings.SplitN(s, "__", 2)
	// Only valid result is a split of two
	if len(parts) != 2 {
		return "", "", false
	}

	namespace := strings.ToLower(parts[0])
	name := strings.ToLower(parts[1])

	return namespace, name, true
}

func (s *ToolsService) RegisterNamespaceMetadata(namespace string, meta NamespaceMetadata) error {
	if namespace == "" {
		return fmt.Errorf("namespace must not be empty")
	}
	key := strings.ToLower(namespace)
	s.nsMeta[key] = meta
	return nil
}

func (s *ToolsService) ListNamespaces() []NamespaceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.namespaces))
	for ns := range s.namespaces {
		names = append(names, ns)
	}
	sort.Strings(names)

	out := make([]NamespaceInfo, 0, len(names))
	for _, ns := range names {
		tools := s.namespaces[ns]
		defs := make([]plugins.ToolDefinition, 0, len(tools))
		for _, t := range tools {
			defs = append(defs, t.ToolDefinition)
		}
		sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })

		meta := s.nsMeta[ns]
		out = append(out, NamespaceInfo{
			Namespace:   ns,
			Description: meta.Description,
			Version:     meta.Version,
			Plugin:      meta.Plugin,
			Builtin:     meta.Builtin,
			System:      meta.System,
			Tools:       defs,
		})
	}
	return out
}

func (s *ToolsService) GetAllToolCalls() ([]plugins.ToolCall, error) {
	calls := make([]plugins.ToolCall, 0)
	for _, namespace := range s.namespaces {
		for _, tool := range namespace {
			calls = append(calls, plugins.ToolCall{
				Name:        tool.FullName,
				Description: tool.ToolDefinition.Description,
				Parameters:  tool.ToolDefinition.Parameters,
			})
		}
	}
	return calls, nil
}

var _ ToolsRegistar = (*ToolsService)(nil)
