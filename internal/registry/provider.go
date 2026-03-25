package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/mwantia/forge/pkg/plugins"
)

type PluginProviderNamespace struct {
	registry *PluginRegistry
}

func (p *PluginProviderNamespace) Chat(ctx context.Context, fullName string, messages []plugins.ChatMessage, tools []plugins.ToolCall) (plugins.ChatStream, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid model format, expected '<provider>/<model>', got '%s'", fullName)
	}

	driver, ok := p.registry.drivers[parts[0]]
	if !ok {
		return nil, fmt.Errorf("unknown driver name defined")
	}

	if driver.Capabilities == nil || driver.Capabilities.Provider == nil {
		return nil, fmt.Errorf("invalid driver - capabilities for provider is missing")
	}

	provider, err := driver.Driver.GetProviderPlugin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load provider: %w", err)
	}

	return provider.Chat(ctx, messages, tools, &plugins.Model{
		ModelName:   parts[1],
		Temperature: 0.7,
	})
}
