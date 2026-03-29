package plugins

import (
	"context"

	"github.com/mwantia/forge/pkg/errors"
)

// SandboxPlugin is the isolation-layer interface.
// Implementations include: builtin (go-landlock), Docker, Podman, remote SSH, etc.
type SandboxPlugin interface {
	BasePlugin

	// CreateSandbox creates a new isolated environment from the given spec.
	// Returns a handle whose ID callers use for all subsequent operations.
	CreateSandbox(ctx context.Context, spec SandboxSpec) (*SandboxHandle, error)

	// DestroySandbox tears down a sandbox and releases all its resources.
	DestroySandbox(ctx context.Context, id string) error

	// CopyIn copies a file or directory from the host into the sandbox.
	CopyIn(ctx context.Context, id, hostSrc, sandboxDst string) error

	// CopyOut copies a file or directory from the sandbox to the host.
	CopyOut(ctx context.Context, id, sandboxSrc, hostDst string) error

	// Execute runs a command inside the sandbox and streams output chunks.
	Execute(ctx context.Context, req SandboxExecRequest) (<-chan SandboxExecChunk, error)

	// Stat checks whether a path exists inside the sandbox and returns basic info.
	Stat(ctx context.Context, id, path string) (*SandboxStatResult, error)

	// ReadFile reads the full content of a file from inside the sandbox.
	ReadFile(ctx context.Context, id, path string) ([]byte, error)
}

// --- Spec and Handle ---

// SandboxSpec describes the desired configuration for a new sandbox.
type SandboxSpec struct {
	// Name is a human-readable label used in log messages.
	Name string `json:"name"`

	// AllowedHostPaths lists host paths the sandbox process may access.
	// Only used by filesystem-isolation backends (e.g. landlock).
	AllowedHostPaths []SandboxPathRule `json:"allowed_host_paths,omitempty"`

	// WorkDir is the working directory for command execution inside the sandbox.
	WorkDir string `json:"work_dir,omitempty"`

	// Env is the base environment variables for command execution.
	Env map[string]string `json:"env,omitempty"`

	// Metadata holds driver-specific extra configuration.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// SandboxPathRule describes a host path the sandbox may access and how.
type SandboxPathRule struct {
	Path     string `json:"path"`
	Writable bool   `json:"writable,omitempty"`
}

// SandboxHandle is returned by CreateSandbox and used to reference an active sandbox.
type SandboxHandle struct {
	// ID is the opaque sandbox identifier assigned by the plugin.
	ID string `json:"id"`
	// Driver is the name of the plugin that owns this handle.
	Driver string `json:"driver"`
}

// --- Execute ---

// SandboxExecRequest describes a command to run inside a sandbox.
type SandboxExecRequest struct {
	// SandboxID is the handle ID returned by CreateSandbox.
	SandboxID string `json:"sandbox_id"`

	// Command is the executable to run.
	Command string `json:"command"`

	// Args are the command-line arguments.
	Args []string `json:"args,omitempty"`

	// Env overrides or extends the sandbox's base environment for this execution.
	Env map[string]string `json:"env,omitempty"`

	// TimeoutSeconds limits execution duration. 0 means use the driver default.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
}

// SandboxExecChunk is a single streamed unit of execution output.
type SandboxExecChunk struct {
	// Stream is "stdout" or "stderr".
	Stream string `json:"stream,omitempty"`

	// Data is a UTF-8 chunk of output.
	Data string `json:"data,omitempty"`

	// ExitCode is set only on the final chunk (Done == true).
	ExitCode int `json:"exit_code,omitempty"`

	// Done marks the final chunk.
	Done bool `json:"done,omitempty"`

	// IsError is true when the chunk represents a framework-level error
	// (e.g. the process could not be started), not a stderr write.
	IsError bool `json:"is_error,omitempty"`
}

// --- Stat ---

// SandboxStatResult is the result of a Stat call.
type SandboxStatResult struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	IsDir   bool   `json:"is_dir,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Mode    string `json:"mode,omitempty"`
	ModTime string `json:"mod_time,omitempty"`
}

// --- Capabilities ---

// SandboxCapabilities describes what a sandbox plugin supports.
type SandboxCapabilities struct {
	// IsolationMode describes the mechanism: "landlock", "container", "vm", "remote".
	IsolationMode string `json:"isolation_mode"`

	// SupportsStreaming indicates whether Execute returns real-time output chunks.
	SupportsStreaming bool `json:"supports_streaming"`

	// SupportsFilesystem indicates whether CopyIn/CopyOut/ReadFile are supported.
	SupportsFilesystem bool `json:"supports_filesystem"`
}

// --- Unimplemented ---

// UnimplementedSandboxPlugin can be embedded to satisfy SandboxPlugin with stub
// error returns, allowing partial implementations.
type UnimplementedSandboxPlugin struct{}

func (UnimplementedSandboxPlugin) GetLifecycle() Lifecycle { return nil }

func (UnimplementedSandboxPlugin) CreateSandbox(_ context.Context, _ SandboxSpec) (*SandboxHandle, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedSandboxPlugin) DestroySandbox(_ context.Context, _ string) error {
	return errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedSandboxPlugin) CopyIn(_ context.Context, _, _, _ string) error {
	return errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedSandboxPlugin) CopyOut(_ context.Context, _, _, _ string) error {
	return errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedSandboxPlugin) Execute(_ context.Context, _ SandboxExecRequest) (<-chan SandboxExecChunk, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedSandboxPlugin) Stat(_ context.Context, _, _ string) (*SandboxStatResult, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}

func (UnimplementedSandboxPlugin) ReadFile(_ context.Context, _, _ string) ([]byte, error) {
	return nil, errors.ErrPluginCapabilityNotSupported
}
