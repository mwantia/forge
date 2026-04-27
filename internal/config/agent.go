package config

import (
	"github.com/hashicorp/hcl/v2"
)

type AgentConfig struct {
	PluginDir string `hcl:"plugin_dir,optional"`

	// Meta is decoded first so its values are available as meta.* in all other blocks.
	Meta *MetaConfig `hcl:"meta,block"`

	// Remain captures all service blocks not declared above (server {}, metrics {}, etc.)
	// so the ConfigTagProcessor can decode them dynamically into service-local config structs.
	Remain hcl.Body `hcl:",remain"`
}
