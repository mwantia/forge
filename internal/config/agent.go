package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mwantia/forge/internal/config/eval"
)

type AgentConfig struct {
	PluginDir string `hcl:"plugin_dir,optional"`

	Storage *StorageConfig `hcl:"storage,block"`
	Server  *ServerConfig  `hcl:"server,block"`
	Metrics *MetricsConfig `hcl:"metrics,block"`

	Plugins []*PluginConfig `hcl:"plugin,block"`
}

func NewDefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
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
	cfg := NewDefaultAgentConfig()
	if path == "" {
		return cfg, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return cfg, fmt.Errorf("unable to access config path '%s': %w", path, err)
	}

	ctx := eval.NewEvalContext(nil)

	if info.IsDir() {
		return parseDir(path, ctx, cfg)
	}

	if err := hclsimple.DecodeFile(path, ctx, cfg); err != nil {
		return cfg, fmt.Errorf("error parsing config '%s': %w", path, err)
	}

	return cfg, nil
}

func parseDir(dir string, ctx *hcl.EvalContext, cfg *AgentConfig) (*AgentConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return cfg, fmt.Errorf("unable to read config directory '%s': %w", dir, err)
	}

	var hclFiles []*hcl.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".hcl") {
			continue
		}

		fpath := filepath.Join(dir, entry.Name())
		src, err := os.ReadFile(fpath)
		if err != nil {
			return cfg, fmt.Errorf("unable to read config file '%s': %w", fpath, err)
		}

		f, diags := hclsyntax.ParseConfig(src, fpath, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			return cfg, fmt.Errorf("error parsing config '%s': %s", fpath, diags.Error())
		}

		hclFiles = append(hclFiles, f)
	}

	if len(hclFiles) == 0 {
		return cfg, nil
	}

	merged := hcl.MergeFiles(hclFiles)
	if diags := gohcl.DecodeBody(merged, ctx, cfg); diags.HasErrors() {
		return cfg, fmt.Errorf("error decoding config directory '%s': %s", dir, diags.Error())
	}

	return cfg, nil
}
