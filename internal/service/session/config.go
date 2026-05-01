package session

// SessionConfig is the HCL config block for the session service.
type SessionConfig struct {
	// DefaultSystem is the seed value used for SessionMetadata.System when a
	// session is created without an explicit `system` field on the request.
	// May reference any session.* template variable (id, name, title,
	// description, parent, model, created_at, updated_at). Empty leaves
	// the session-layer prompt empty and skipped during assembly.
	DefaultSystem string `hcl:"default_system,optional"`
}
