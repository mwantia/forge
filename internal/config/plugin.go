package config

import "github.com/hashicorp/hcl/v2"

type PluginConfig struct {
	Name   string            `hcl:"name,label"`
	Config *PluginConfigBody `hcl:"config,block"`
}

type PluginConfigBody struct {
	Body hcl.Body `hcl:",remain"`
}