package session

// SessionConfig is the HCL config block for the session service.
type SessionConfig struct {
	// Backend selects the session store implementation. Empty defaults to "file".
	// "plugin" delegates session and message storage to a SessionsPlugin.
	Backend string `hcl:"backend,optional"`

	// Plugin is the block name of the SessionsPlugin to bind to when
	// Backend = "plugin". Required in that case.
	Plugin string `hcl:"plugin,optional"`

	// DefaultSystem is the seed value used for SessionMetadata.System when a
	// session is created without an explicit `system` field on the request.
	// May reference any session.* template variable (id, name, title,
	// description, parent, model, created_at, updated_at). Empty leaves
	// the session-layer prompt empty and skipped during assembly.
	DefaultSystem string `hcl:"default_system,optional"`
}

const (
	BackendFile   = "file"
	BackendPlugin = "plugin"
)
