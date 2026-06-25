package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	plugins "github.com/mwantia/forge-sdk/pkg/plugin"
	"github.com/mwantia/forge-sdk/pkg/plugin/provider"
	domtool "github.com/mwantia/forge/internal/domain/tool"
	inframetrics "github.com/mwantia/forge/internal/infrastructure/metrics"
)

type (
	ToolsRegistar     = domtool.ToolsRegistar
	NamespaceMetadata = domtool.NamespaceMetadata
	NamespaceInfo     = domtool.NamespaceInfo
	ToolsExecution    = domtool.ToolsExecution
)

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
	s.mu.Lock()
	defer s.mu.Unlock()

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

func (s *ToolsService) ExecuteToolWithCallID(ctx context.Context, namespace, name string, arguments map[string]any, callID string) (*plugins.ExecuteToolResponse, error) {
	tool, ok := s.getToolNamespace(namespace, name)
	if !ok {
		return nil, fmt.Errorf("tool with namespace %q and name %q not found", namespace, name)
	}

	start := time.Now()
	resp, err := tool.Execution(ctx, plugins.ExecuteToolRequest{
		Tool:   tool.ToolDefinition.Name,
		Args:   plugins.NewToolArgs(arguments),
		CallID: callID,
	})

	ToolsExecutionDuration.WithLabelValues(namespace, name).Observe(time.Since(start).Seconds())
	status := inframetrics.ErrToStatusLabel(err)
	if err == nil && resp != nil && !resp.Success {
		status = "error"
	}
	ToolsExecutionsTotal.WithLabelValues(namespace, name, status).Inc()

	return resp, err
}

func (s *ToolsService) ExecuteTool(ctx context.Context, namespace, name string, arguments map[string]any) (*plugins.ExecuteToolResponse, error) {
	return s.ExecuteToolWithCallID(ctx, namespace, name, arguments, "")
}

func (s *ToolsService) GetToolDefinition(namespace, name string) (plugins.ToolDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tool, ok := s.getToolNamespace(namespace, name)
	if ok {
		return tool.ToolDefinition, nil
	}

	return plugins.ToolDefinition{}, fmt.Errorf("tool definition with namespace %q and name %q not found", namespace, name)
}

func (s *ToolsService) GetToolDefinitionsByNamespace(namespace string) ([]plugins.ToolDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
	s.mu.RLock()
	defer s.mu.RUnlock()

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
	s.mu.RLock()
	defer s.mu.RUnlock()

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

func (s *ToolsService) GetAllToolCalls() ([]provider.ToolCall, error) {
	calls := make([]provider.ToolCall, 0)
	for _, namespace := range s.namespaces {

		for _, tool := range namespace {
			calls = append(calls, provider.ToolCall{
				Name:        tool.FullName,
				Description: tool.ToolDefinition.Description,
				Parameters:  tool.ToolDefinition.Parameters,
			})
		}
	}

	return calls, nil
}

var _ ToolsRegistar = (*ToolsService)(nil)
