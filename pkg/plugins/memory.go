package plugins

import (
	"context"

	"github.com/mwantia/forge/pkg/errors"
)

// MemoryPlugin acts as memory management for endpoints like OpenViking.
type MemoryPlugin interface {
	BasePlugin

	StoreResource(ctx context.Context, sessionID, content string, metadata map[string]any) (*MemoryResource, error)
	RetrieveResource(ctx context.Context, sessionID, query string, limit int, filter map[string]any) ([]*MemoryResource, error)

	CreateSession(ctx context.Context) (*MemorySession, error)
	GetSession(ctx context.Context, sessionID string) (*MemorySession, error)
	ListSessions(ctx context.Context) ([]*MemorySession, error)
	DeleteSession(ctx context.Context, sessionID string) (bool, error)
	CommitSession(ctx context.Context, sessionID string) (bool, error)

	AddMessage(ctx context.Context, sessionID, role, content string) (bool, error)
}

type MemoryResource struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Score    float64        `json:"score"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type MemorySession struct {
	ID           string `json:"id"`
	Author       string `json:"author"`
	Committed    bool   `json:"committed,omitempty"`
	Archived     bool   `json:"archived,omitempty"`
	MessageCount int    `json:"message_count"`
}

type StoreRequest struct {
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Namespace string         `json:"namespace,omitempty"`
}

// UnimplementedMemoryPlugin can be embedded to satisfy MemoryPlugin with default
// implementations that return errors.ErrPluginCapabilityNotSupported.
type UnimplementedMemoryPlugin struct{}

func (UnimplementedMemoryPlugin) GetLifecycle() Lifecycle { return nil }

func (UnimplementedMemoryPlugin) StoreResource(_ context.Context, _, _ string, _ map[string]any) (*MemoryResource, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedMemoryPlugin) RetrieveResource(_ context.Context, _, _ string, _ int, _ map[string]any) ([]*MemoryResource, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedMemoryPlugin) CreateSession(_ context.Context) (*MemorySession, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedMemoryPlugin) GetSession(_ context.Context, _ string) (*MemorySession, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedMemoryPlugin) ListSessions(_ context.Context) ([]*MemorySession, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedMemoryPlugin) DeleteSession(_ context.Context, _ string) (bool, error) {
	return false, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedMemoryPlugin) CommitSession(_ context.Context, _ string) (bool, error) {
	return false, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedMemoryPlugin) AddMessage(_ context.Context, _, _, _ string) (bool, error) {
	return false, errors.ErrPluginCapabilityNotSupported
}
