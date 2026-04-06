package sandbox

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge-sdk/pkg/random"
	"github.com/mwantia/forge/internal/registry"
	"github.com/mwantia/forge/internal/storage"
)

const defaultIsolationDriver = "builtin"

// Manager handles sandbox lifecycle: creation, persistence, and dispatch to plugins.
type SandboxManager struct {
	log      hclog.Logger             `fabric:"logger:sandbox"`
	registry *registry.PluginRegistry `fabric:"inject"`
	backend  storage.Backend          `fabric:"inject"`

	mu      sync.RWMutex
	handles map[string]*activeHandle // sandboxID → active handle
}

// activeHandle pairs a live plugin with the handle it returned for a sandbox.
type activeHandle struct {
	plugin plugins.SandboxPlugin
	handle plugins.SandboxHandle
}

// CreateOptions is the request body for creating a sandbox.
type CreateOptions struct {
	Name            string              `json:"name,omitempty"`
	SessionID       string              `json:"session_id"`
	IsolationDriver string              `json:"isolation_driver,omitempty"`
	Spec            plugins.SandboxSpec `json:"spec,omitempty"`
}

// Create creates a new sandbox, persists its record, and invokes the plugin.
func (m *SandboxManager) Create(ctx context.Context, opts CreateOptions) (*Sandbox, error) {
	if opts.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	driver := opts.IsolationDriver
	if driver == "" {
		driver = defaultIsolationDriver
	}

	plugin, err := m.registry.GetSandboxPlugin(ctx, driver)
	if err != nil {
		return nil, fmt.Errorf("isolation driver %q not available: %w", driver, err)
	}

	id := random.GenerateNewID()
	name := opts.Name
	if name == "" {
		name = id[:8]
	}

	spec := opts.Spec
	spec.Name = name

	now := time.Now()
	sb := &Sandbox{
		ID:              id,
		Name:            name,
		SessionID:       opts.SessionID,
		IsolationDriver: driver,
		Status:          StatusCreating,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := m.saveSandbox(sb); err != nil {
		return nil, fmt.Errorf("failed to persist sandbox record: %w", err)
	}

	handle, err := plugin.CreateSandbox(ctx, spec)
	if err != nil {
		sb.Status = StatusError
		_ = m.saveSandbox(sb)
		return nil, fmt.Errorf("failed to create sandbox via plugin: %w", err)
	}

	sb.Status = StatusReady
	sb.UpdatedAt = time.Now()
	if err := m.saveSandbox(sb); err != nil {
		m.log.Warn("Failed to update sandbox status after creation", "id", id, "error", err)
	}

	m.mu.Lock()
	m.handles[id] = &activeHandle{plugin: plugin, handle: *handle}
	m.mu.Unlock()

	m.log.Info("Sandbox created", "id", id, "name", name, "driver", driver, "session", opts.SessionID)
	return sb, nil
}

// Get returns the persisted sandbox record by ID.
func (m *SandboxManager) Get(id string) (*Sandbox, error) {
	return m.loadSandboxByID(id)
}

// List returns sandboxes matching the given options.
func (m *SandboxManager) List(opts ListOptions) ([]*Sandbox, error) {
	return m.listSandboxes(opts)
}

// Delete destroys the sandbox via the plugin and removes its record.
func (m *SandboxManager) Delete(ctx context.Context, id string) error {
	sb, err := m.loadSandboxByID(id)
	if err != nil {
		return err
	}

	m.mu.Lock()
	ah, active := m.handles[id]
	if active {
		delete(m.handles, id)
	}
	m.mu.Unlock()

	if active {
		if err := ah.plugin.DestroySandbox(ctx, ah.handle.ID); err != nil {
			m.log.Warn("Failed to destroy sandbox via plugin", "id", id, "error", err)
		}
	}

	return m.deleteSandbox(sb.SessionID, id)
}

// NewToolsPlugin returns a ToolsPlugin that exposes sandbox operations as agent tools
// scoped to the given session. This satisfies the session.SandboxManagerIface.
func (m *SandboxManager) NewToolsPlugin(sessionID string) plugins.ToolsPlugin {
	return &SandboxToolsPlugin{Manager: m, SessionID: sessionID}
}

// DeleteBySession destroys all sandboxes belonging to a session.
// Called before deleting a session so OS-level resources are released.
func (m *SandboxManager) DeleteBySession(ctx context.Context, sessionID string) error {
	sbs, err := m.listSandboxes(ListOptions{SessionID: sessionID})
	if err != nil {
		return err
	}
	for _, sb := range sbs {
		if delErr := m.Delete(ctx, sb.ID); delErr != nil {
			m.log.Warn("Failed to delete sandbox during session cleanup", "sandbox", sb.ID, "error", delErr)
		}
	}
	return nil
}

// UpdateStatus updates only the status field on disk.
func (m *SandboxManager) UpdateStatus(id string, status SandboxStatus) error {
	sb, err := m.loadSandboxByID(id)
	if err != nil {
		return err
	}
	sb.Status = status
	sb.UpdatedAt = time.Now()
	return m.saveSandbox(sb)
}

// --- Plugin operation dispatch ---

func (m *SandboxManager) getActiveHandle(id string) (*activeHandle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ah, ok := m.handles[id]
	if !ok {
		return nil, fmt.Errorf("sandbox %q is not active (may have been stopped or server restarted)", id)
	}
	return ah, nil
}

func (m *SandboxManager) CopyIn(ctx context.Context, id, hostSrc, sandboxDst string) error {
	ah, err := m.getActiveHandle(id)
	if err != nil {
		return err
	}
	return ah.plugin.CopyIn(ctx, ah.handle.ID, hostSrc, sandboxDst)
}

func (m *SandboxManager) CopyOut(ctx context.Context, id, sandboxSrc, hostDst string) error {
	ah, err := m.getActiveHandle(id)
	if err != nil {
		return err
	}
	return ah.plugin.CopyOut(ctx, ah.handle.ID, sandboxSrc, hostDst)
}

func (m *SandboxManager) Execute(ctx context.Context, id string, req plugins.SandboxExecRequest) (<-chan plugins.SandboxExecChunk, error) {
	ah, err := m.getActiveHandle(id)
	if err != nil {
		return nil, err
	}
	req.SandboxID = ah.handle.ID
	return ah.plugin.Execute(ctx, req)
}

func (m *SandboxManager) Stat(ctx context.Context, id, path string) (*plugins.SandboxStatResult, error) {
	ah, err := m.getActiveHandle(id)
	if err != nil {
		return nil, err
	}
	return ah.plugin.Stat(ctx, ah.handle.ID, path)
}

func (m *SandboxManager) ReadFile(ctx context.Context, id, path string) ([]byte, error) {
	ah, err := m.getActiveHandle(id)
	if err != nil {
		return nil, err
	}
	return ah.plugin.ReadFile(ctx, ah.handle.ID, path)
}
