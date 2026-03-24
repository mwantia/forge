package stub

import (
	"context"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
)

// StubMemoryPlugin implements MemoryPlugin.
type StubMemoryPlugin struct {
	plugins.UnimplementedMemoryPlugin
	driver *StubDriver
}

func (p *StubMemoryPlugin) GetLifecycle() plugins.Lifecycle {
	return p.driver
}

func (p *StubMemoryPlugin) StoreResource(_ context.Context, sessionID, content string, _ map[string]any) (*plugins.MemoryResource, error) {
	return &plugins.MemoryResource{
		ID:      fmt.Sprintf("stub-%s-resource", sessionID),
		Content: content,
		Score:   1.0,
	}, nil
}

func (p *StubMemoryPlugin) RetrieveResource(_ context.Context, _ string, _ string, _ int, _ map[string]any) ([]*plugins.MemoryResource, error) {
	return []*plugins.MemoryResource{
		{
			ID:      "stub-resource-id",
			Content: "This is a stub memory result.",
			Score:   1.0,
		},
	}, nil
}

func (p *StubMemoryPlugin) CreateSession(_ context.Context) (*plugins.MemorySession, error) {
	return &plugins.MemorySession{ID: "stub-session-id", Author: "stub"}, nil
}

func (p *StubMemoryPlugin) GetSession(_ context.Context, sessionID string) (*plugins.MemorySession, error) {
	return &plugins.MemorySession{ID: sessionID, Author: "stub"}, nil
}

func (p *StubMemoryPlugin) ListSessions(_ context.Context) ([]*plugins.MemorySession, error) {
	return []*plugins.MemorySession{
		{ID: "stub-session-id", Author: "stub"},
	}, nil
}

func (p *StubMemoryPlugin) DeleteSession(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (p *StubMemoryPlugin) CommitSession(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (p *StubMemoryPlugin) AddMessage(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}
