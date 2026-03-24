package skills

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/mwantia/forge/pkg/errors"
	"github.com/mwantia/forge/pkg/plugins"
)

const PluginName = "skills"

func init() {
	plugins.Register(PluginName, NewSkillsDriver)
}

// SkillsDriver implements plugins.Driver for the skills plugin.
type SkillsToolsDriver struct {
	log    hclog.Logger
	config *SkillsToolsConfig
	skills map[string]*Skill
}

type SkillsToolsConfig struct {
	Path string `mapstructure:"path"`
}

// NewSkillsDriver creates a new skills driver that supports tools plugin type.
func NewSkillsDriver(log hclog.Logger) plugins.Driver {
	return &SkillsToolsDriver{
		log: log.Named(PluginName),
	}
}

// Lifecycle methods
func (d *SkillsToolsDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:    PluginName,
		Author:  "forge",
		Version: "0.1.0",
	}
}

func (d *SkillsToolsDriver) ProbePlugin(ctx context.Context) (bool, error) {
	return true, nil
}

func (d *SkillsToolsDriver) GetCapabilities(ctx context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeTools},
		Tools: &plugins.ToolsCapabilities{
			SupportsAsyncExecution: false,
		},
	}, nil
}

func (d *SkillsToolsDriver) OpenDriver(ctx context.Context) error {

	return nil
}

func (d *SkillsToolsDriver) CloseDriver(ctx context.Context) error {
	return nil
}

func (d *SkillsToolsDriver) ConfigDriver(ctx context.Context, config plugins.PluginConfig) error {
	if err := mapstructure.Decode(config.ConfigMap, &d.config); err != nil {
		return fmt.Errorf("failed to decode config: %v", err)
	}

	path := d.config.Path
	if path == "" {
		path = "./skills"
	}
	// Validate path exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to access path '%s': %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path '%s' is not a directory", path)
	}

	d.log.Info("Searching SKILL tools", "path", path)
	// Scan and load skills
	skills, err := d.scanSkills(path)
	if err != nil {
		return fmt.Errorf("failed to scan skills: %w", err)
	}
	d.skills = skills

	for _, skill := range skills {
		d.log.Debug("Skill", "name", skill.Name, "desc", skill.Description, "path", skill.Path)
	}

	d.log.Info("Loaded skills", "count", len(skills), "path", path)
	return nil
}

// Plugin type accessors
func (d *SkillsToolsDriver) GetProviderPlugin(ctx context.Context) (plugins.ProviderPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *SkillsToolsDriver) GetMemoryPlugin(ctx context.Context) (plugins.MemoryPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *SkillsToolsDriver) GetChannelPlugin(ctx context.Context) (plugins.ChannelPlugin, error) {
	return nil, errors.ErrPluginNotSupported
}

func (d *SkillsToolsDriver) GetToolsPlugin(ctx context.Context) (plugins.ToolsPlugin, error) {
	return d, nil
}
