package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mwantia/forge/pkg/errors"
	"github.com/mwantia/forge/pkg/plugins"
)

func (p *SkillsToolsDriver) GetLifecycle() plugins.Lifecycle {
	return p
}

// scanSkills scans the directory for SKILL.md files and parses them.
func (p *SkillsToolsDriver) scanSkills(root string) (map[string]*Skill, error) {
	skills := make(map[string]*Skill)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() != "SKILL.md" {
			return nil
		}

		skill, err := p.parseSkillFile(path)
		if err != nil {
			p.log.Warn("Failed to parse skill file", "path", path, "error", err)
			return nil // Continue scanning other files
		}

		skills[skill.Name] = skill
		p.log.Debug("Loaded skill", "name", skill.Name, "path", path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return skills, nil
}

// parseSkillFile parses a SKILL.md file into a Skill struct.
func (p *SkillsToolsDriver) parseSkillFile(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	skill := &Skill{
		Path:       path,
		Parameters: make(map[string]Parameter),
		Content:    string(content),
	}

	// Extract skill name from parent directory
	dir := filepath.Dir(path)
	skill.Name = filepath.Base(dir)

	// Parse frontmatter and content
	frontmatter, body, err := parseFrontmatter(string(content))
	if err == nil && frontmatter != nil {
		// Use frontmatter values
		if name, ok := frontmatter["name"].(string); ok && name != "" {
			skill.Name = name
		}
		if desc, ok := frontmatter["description"].(string); ok {
			skill.Description = desc
		}
		if version, ok := frontmatter["version"].(string); ok {
			skill.Version = version
		}
		if deprecated, ok := frontmatter["deprecated"].(string); ok {
			skill.Deprecated = deprecated == "true"
		}
		if msg, ok := frontmatter["deprecation_message"].(string); ok {
			skill.DeprecationMessage = msg
		}
		if tagsRaw, ok := frontmatter["tags"].(string); ok && tagsRaw != "" {
			for _, tag := range strings.Split(tagsRaw, ",") {
				if t := strings.TrimSpace(tag); t != "" {
					skill.Tags = append(skill.Tags, t)
				}
			}
		}
		if params, ok := frontmatter["parameters"].(map[string]any); ok {
			for paramName, paramDef := range params {
				if param, ok := paramDef.(map[string]any); ok {
					p := Parameter{}
					if t, ok := param["type"].(string); ok {
						p.Type = t
					} else {
						p.Type = "string" // default type
					}
					if d, ok := param["description"].(string); ok {
						p.Description = d
					}
					if r, ok := param["required"].(bool); ok {
						p.Required = r
					}
					if def, ok := param["default"]; ok {
						p.Default = def
					}
					skill.Parameters[paramName] = p
				}
			}
		}
	}

	// Use body as description if not set in frontmatter
	if skill.Description == "" {
		// Use first paragraph or line
		lines := strings.Split(strings.TrimSpace(body), "\n")
		if len(lines) > 0 {
			// Find first non-empty, non-heading line
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					skill.Description = line
					break
				}
			}
		}
	}

	if skill.Name == "" {
		return nil, fmt.Errorf("skill name is required")
	}

	return skill, nil
}

// parseFrontmatter extracts YAML-like frontmatter from markdown content.
func parseFrontmatter(content string) (map[string]any, string, error) {
	// Check for frontmatter delimiter
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return nil, content, fmt.Errorf("no frontmatter found")
	}

	// Find closing delimiter
	delimStart := strings.Index(content, "---")
	if delimStart == -1 {
		return nil, content, fmt.Errorf("no opening delimiter")
	}

	remaining := content[delimStart+3:] // skip opening ---
	if strings.HasPrefix(remaining, "\r\n") {
		remaining = remaining[2:]
	} else if strings.HasPrefix(remaining, "\n") {
		remaining = remaining[1:]
	}

	delimEnd := strings.Index(remaining, "---\n")
	if delimEnd == -1 {
		delimEnd = strings.Index(remaining, "---\r\n")
		if delimEnd == -1 {
			return nil, content, fmt.Errorf("no closing delimiter")
		}
	}

	frontmatterText := strings.TrimSpace(remaining[:delimEnd])
	body := strings.TrimSpace(remaining[delimEnd+3:])

	// Simple YAML-like parsing
	frontmatter := make(map[string]any)
	lines := strings.SplitSeq(frontmatterText, "\n")

	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle key: value
		before, after, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		key := strings.TrimSpace(before)
		value := strings.TrimSpace(after)

		// Simple value parsing (remove quotes)
		value = strings.Trim(value, "\"'")
		frontmatter[key] = value
	}

	return frontmatter, body, nil
}

func (p *SkillsToolsDriver) ListTools(_ context.Context, filter plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	if p.skills == nil {
		return nil, fmt.Errorf("plugin not configured, call SetConfig first")
	}

	tools := make([]plugins.ToolDefinition, 0, len(p.skills))
	for name, skill := range p.skills {
		def := skillToToolDefinition(name, skill)
		if !skillMatchesFilter(def, filter) {
			continue
		}
		p.log.Debug("Tool definition", "name", name, "description", skill.Description, "tags", skill.Tags)
		tools = append(tools, def)
	}

	return &plugins.ListToolsResponse{Tools: tools}, nil
}

func (p *SkillsToolsDriver) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	if p.skills == nil {
		return nil, fmt.Errorf("plugin not configured, call SetConfig first")
	}

	skill, ok := p.skills[name]
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}

	def := skillToToolDefinition(name, skill)
	return &def, nil
}

func (p *SkillsToolsDriver) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	if p.skills == nil {
		return nil, fmt.Errorf("plugin not configured, call SetConfig first")
	}

	skill, ok := p.skills[req.Tool]
	if !ok {
		return &plugins.ValidateResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("skill %q not found", req.Tool)},
		}, nil
	}

	var errs []string
	for paramName, param := range skill.Parameters {
		if !param.Required {
			continue
		}
		v, present := req.Arguments[paramName]
		if !present || v == nil {
			errs = append(errs, fmt.Sprintf("%q is required", paramName))
			continue
		}
		if param.Type == "string" {
			if _, ok := v.(string); !ok {
				errs = append(errs, fmt.Sprintf("%q must be a string", paramName))
			}
		}
	}

	return &plugins.ValidateResponse{Valid: len(errs) == 0, Errors: errs}, nil
}

// skillToToolDefinition converts a Skill to a plugins.ToolDefinition.
func skillToToolDefinition(name string, skill *Skill) plugins.ToolDefinition {
	properties := make(map[string]any)
	var required []string

	for paramName, param := range skill.Parameters {
		propDef := map[string]any{"type": param.Type}
		if param.Description != "" {
			propDef["description"] = param.Description
		}
		if param.Default != nil {
			propDef["default"] = param.Default
		}
		properties[paramName] = propDef
		if param.Required {
			required = append(required, paramName)
		}
	}

	var params map[string]any
	if len(properties) > 0 {
		params = map[string]any{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			params["required"] = required
		}
	}

	return plugins.ToolDefinition{
		Name:               name,
		Description:        skill.Description,
		Parameters:         params,
		Tags:               skill.Tags,
		Version:            skill.Version,
		Deprecated:         skill.Deprecated,
		DeprecationMessage: skill.DeprecationMessage,
		Annotations: plugins.ToolAnnotations{
			CostHint: "cheap",
		},
	}
}

// skillMatchesFilter reports whether def satisfies the given filter.
func skillMatchesFilter(def plugins.ToolDefinition, f plugins.ListToolsFilter) bool {
	if def.Deprecated && !f.Deprecated {
		return false
	}
	if f.Prefix != "" && !strings.HasPrefix(def.Name, f.Prefix) {
		return false
	}
	if len(f.Tags) > 0 {
		for _, want := range f.Tags {
			for _, have := range def.Tags {
				if have == want {
					goto tagMatched
				}
			}
		}
		return false
	tagMatched:
	}
	return true
}

func (p *SkillsToolsDriver) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	if p.skills == nil {
		return nil, fmt.Errorf("plugin not configured, call SetConfig first")
	}

	skill, ok := p.skills[req.Tool]
	if !ok {
		return nil, errors.ErrSkillNotFound
	}

	// Return the skill content for execution
	// The actual execution logic would be implemented by the agent/LLM
	result := map[string]any{
		"skill":     skill.Name,
		"content":   skill.Content,
		"path":      skill.Path,
		"arguments": req.Arguments,
		"executed":  true,
		"message":   fmt.Sprintf("Skill '%s' executed successfully", skill.Name),
	}

	return &plugins.ExecuteResponse{
		Result:  result,
		IsError: false,
	}, nil
}
