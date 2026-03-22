package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

type AgentConfig struct {
	LogLevel  string `hcl:"log_level,optional"`
	PluginDir string `hcl:"plugin_dir,optional"`

	Server  *ServerConfig  `hcl:"server,block"`
	Metrics *MetricsConfig `hcl:"metrics,block"`
	Plugins []*PluginConfig `hcl:"plugin,block"`
}

func NewDefault() *AgentConfig {
	return &AgentConfig{
		LogLevel: "INFO",
		Server: &ServerConfig{
			Address: "127.0.0.1:9280",
			Token:   "",
		},
		Metrics: &MetricsConfig{
			Address: "127.0.0.1:9500",
		},
	}
}

func Parse(path string) (*AgentConfig, error) {
	cfg := NewDefault()
	if path == "" {
		return cfg, nil
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return cfg, fmt.Errorf("unable to access config file '%s': %w", path, err)
	}

	if err := hclsimple.DecodeFile(path, nil, cfg); err != nil {
		return cfg, fmt.Errorf("error parsing config '%s': %w", path, err)
	}

	return cfg, nil
}
