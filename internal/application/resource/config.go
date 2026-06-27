package resource

import (
	"fmt"
	"strings"

	"github.com/mwantia/forge-sdk/pkg/data"
)

const (
	DefaultResourceEmbedTemplate    = "{{ .embed }}"
	DefaultResourceUploadOptimize   = false
	DefaultResourceUploadExtensions = ".md,.txt"
	DefaultResourceUploadFilesize   = 1 * 1024 * 1024 // 1MB
	DefaultResourceSummaryTemplate  = `
	Analyse the provided resource content and generate a tiered summary adhering to the provided guidelines.
    Provide a structuted overview that summarizes the core content with a maximum of a 100 words total.
    Crucial key informations and highlights need to be preserved - The summary is used to decide relevancy for tasks and research operations. 
	`
)

type ResourceConfig struct {
	Embed   *ResourceEmbedConfig   `hcl:"embed,block"`
	Upload  *ResourceUploadConfig  `hcl:"upload,block"`
	Summary *ResourceSummaryConfig `hcl:"summary,block"`
}

type ResourceEmbedConfig struct {
	Model    string `hcl:"model,optional"`
	Template string `hcl:"template,optional"`
}

func (c *ResourceEmbedConfig) GetModel() string {
	if c == nil {
		return ""
	}

	return c.Model
}

func (c *ResourceEmbedConfig) GetTemplate() string {
	if c == nil {
		return DefaultResourceEmbedTemplate
	}

	return c.Template
}

type ResourceUploadConfig struct {
	Filesize   string `hcl:"filesize,optional"`
	Optimize   *bool  `hcl:"optimize,optional"`
	Extensions string `hcl:"extensions,optional"`
}

func (c *ResourceUploadConfig) GetFilesize() (uint64, error) {
	if c == nil || c.Filesize == "" {
		return DefaultResourceUploadFilesize, nil
	}

	size, err := data.ParseBytes(c.Filesize)
	if err != nil {
		return 0, fmt.Errorf("failed to parse 'filesize' from value %q: %w", c.Filesize, err)
	}

	return size, nil
}

func (c *ResourceUploadConfig) GetOptimize() bool {
	if c == nil || c.Optimize == nil {
		return DefaultResourceUploadOptimize
	}

	return *c.Optimize
}

func (c *ResourceUploadConfig) GetExtensions() []string {
	if c == nil || c.Extensions == "" {
		return strings.Split(DefaultResourceUploadExtensions, ",")
	}

	return strings.Split(c.Extensions, ",")
}

type ResourceSummaryConfig struct {
	Model    string `hcl:"model,optional"`
	Template string `hcl:"template,optional"`
}

func (c *ResourceSummaryConfig) GetModel() string {
	if c == nil {
		return ""
	}

	return c.Model
}

func (c *ResourceSummaryConfig) GetTemplate() string {
	if c == nil || c.Template == "" {
		return DefaultResourceSummaryTemplate
	}

	return c.Template
}
