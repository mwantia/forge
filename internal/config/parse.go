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
	"github.com/mwantia/forge/internal/service/template"
)

func Parse(path string, tmpl *template.Template) (*AgentConfig, error) {
	cfg := &AgentConfig{}
	if path == "" {
		return cfg, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return cfg, fmt.Errorf("unable to access config path '%s': %w", path, err)
	}

	ctx := tmpl.Eval()

	if info.IsDir() {
		return parseDir(path, ctx, cfg)
	}

	if err := hclsimple.DecodeFile(path, ctx, cfg); err != nil {
		return cfg, fmt.Errorf("error parsing config '%s': %w", path, err)
	}

	// TODO :: Find a solution to dynamically register 'meta' for sub-blocks
	/* if !eval.RegisterBodyAsVariables("meta", cfg.Meta.Body) {
		return cfg, fmt.Errorf("failed to register meta as accessor variables")
	} */

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

	merged, err := mergeHCLSyntaxBodies(hclFiles)
	if err != nil {
		return cfg, fmt.Errorf("error merging config files in '%s': %w", dir, err)
	}
	if diags := gohcl.DecodeBody(merged, ctx, cfg); diags.HasErrors() {
		return cfg, fmt.Errorf("error decoding config directory '%s': %s", dir, diags.Error())
	}

	return cfg, nil
}

func mergeHCLSyntaxBodies(files []*hcl.File) (*hclsyntax.Body, error) {
	merged := &hclsyntax.Body{
		Attributes: make(hclsyntax.Attributes),
	}
	for _, f := range files {
		syn, ok := f.Body.(*hclsyntax.Body)
		if !ok {
			return nil, fmt.Errorf("expected *hclsyntax.Body from HCL file, got %T", f.Body)
		}
		for name, attr := range syn.Attributes {
			merged.Attributes[name] = attr
		}
		merged.Blocks = append(merged.Blocks, syn.Blocks...)
	}
	return merged, nil
}
