package plugins

// BasePlugin is the common interface all sub-plugins share.
// Sub-plugins reference their parent driver via GetLifecycle.
type BasePlugin interface {
	GetLifecycle() Lifecycle
}
