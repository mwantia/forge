package resource

type ResourceConfig struct {
	// EmbedModel is a forge model alias (e.g. "forge/nomic") used for
	// semantic indexing and recall. Requires a provider plugin that supports
	// embeddings (e.g. ollama with nomic-embed-text, openai).
	EmbedModel string `hcl:"embed_model,optional"`
}
