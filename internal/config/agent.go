package config

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/mwantia/forge/internal/log"
)

type AgentConfig struct {
	PluginDir       string        `hcl:"plugin_dir,optional"`
	Log             log.LogConfig `hcl:"log,block"`
	ShutdownTimeout string        `hcl:"shutdown_timeout,optional"`

	// Remain captures all service blocks not declared above (server {}, metrics {}, etc.)
	// so the ConfigTagProcessor can decode them dynamically into service-local config structs.
	Remain hcl.Body `hcl:",remain"`
}
