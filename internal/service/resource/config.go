package resource

type ResourceConfig struct {
	// Mounts declares path-to-plugin bindings. Longest prefix wins.
	// Paths not matching any mount fall through to the built-in file store.
	Mounts []*ResourceMountConfig `hcl:"mount,block"`

	// EmbedModel is a forge model alias (e.g. "forge/nomic") used for
	// embedding augmentation. Tolerated here so config files can declare it
	// without parse errors.
	EmbedModel string `hcl:"embed_model,optional"`
}

type ResourceMountConfig struct {
	// Plugin is the name of a resource-capable plugin driver to bind at this
	// prefix. Empty (or "file") selects the built-in file store.
	Plugin string `hcl:"plugin,label"`

	// Path is the path prefix this mount owns (e.g. "/global", "/archives").
	// Use "/" for a catch-all.
	Path string `hcl:"path,optional"`
}
