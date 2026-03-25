package skills

// Skill represents a parsed skill from a SKILL.md file.
type Skill struct {
	Name               string
	Description        string
	Parameters         map[string]Parameter
	Tags               []string
	Version            string
	Deprecated         bool
	DeprecationMessage string
	Content            string
	Path               string
}

// Parameter represents a tool parameter definition.
type Parameter struct {
	Type        string
	Description string
	Required    bool
	Default     any
}
