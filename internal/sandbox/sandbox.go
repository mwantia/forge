package sandbox

import "time"

// SandboxStatus represents the lifecycle state of a sandbox.
type SandboxStatus string

const (
	StatusCreating SandboxStatus = "creating"
	StatusReady    SandboxStatus = "ready"
	StatusStopped  SandboxStatus = "stopped"
	StatusError    SandboxStatus = "error"
)

// Sandbox is the persisted record of a sandbox instance.
type Sandbox struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	SessionID       string            `json:"session_id"`
	IsolationDriver string            `json:"isolation_driver"`
	Status          SandboxStatus     `json:"status"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Metadata        map[string]any    `json:"metadata,omitempty"`
}
