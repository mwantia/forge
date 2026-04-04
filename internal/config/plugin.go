package config

import (
	"github.com/hashicorp/hcl/v2"
)

type PluginConfig struct {
	Name     string               `hcl:"name,label"`
	Type     string               `hcl:"type,label"`
	Disabled bool                 `hcl:"enabled,optional"`
	Runtime  *PluginRuntimeConfig `hcl:"runtime,block"`
	Config   *PluginConfigConfig  `hcl:"config,block"`
}

type PluginRuntimeConfig struct {
	Path    string                   `hcl:"path,optional"`
	Args    []string                 `hcl:"args,optional"`
	Timeout string                   `hcl:"timeout,optional"`
	Port    *PluginRuntimePortConfig `hcl:"port,block"`
	Env     *PluginRuntimeEnvConfig  `hcl:"env,block"`
}

type PluginRuntimePortConfig struct {
	Min uint `hcl:"min,optional"`
	Max uint `hcl:"max,optional"`
}

type PluginRuntimeEnvConfig struct {
	Body hcl.Body `hcl:",remain"`
}

type PluginConfigConfig struct {
	Body hcl.Body `hcl:",remain"`
}
