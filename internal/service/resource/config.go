package resource

type ResourceConfig struct {
	// Plugin is the block name of the resource-capable plugin to bind to.
	// When empty the first driver exposing ResourceCapabilities is chosen.
	Plugin string `hcl:"plugin,optional"`

	// DefaultPath is used when a caller omits a path and no caller session
	// ID is available in the context.
	DefaultPath string `hcl:"default_path,optional"`

	// EmbedModel is a forge model alias (e.g. "forge/nomic") used for
	// embedding augmentation. Wired up in a later phase; tolerated here so
	// config files can declare it without parse errors.
	EmbedModel string `hcl:"embed_model,optional"`
}
