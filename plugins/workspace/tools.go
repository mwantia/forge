package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mwantia/forge/pkg/plugins"
)

func (d *WorkspaceDriver) GetLifecycle() plugins.Lifecycle {
	return d
}

func (d *WorkspaceDriver) ListTools(_ context.Context, filter plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	tools := make([]plugins.ToolDefinition, 0, len(d.config.Tools))
	for _, name := range d.config.Tools {
		def, ok := toolDefinitions[name]
		if !ok {
			d.log.Warn("Unknown tool in config, skipping", "tool", name)
			continue
		}
		if matchesFilter(def, filter) {
			tools = append(tools, def)
		}
	}

	return &plugins.ListToolsResponse{Tools: tools}, nil
}

func (d *WorkspaceDriver) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	def, ok := toolDefinitions[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}

	// Only expose tools that are enabled in the config.
	for _, enabled := range d.config.Tools {
		if enabled == name {
			return &def, nil
		}
	}
	return nil, fmt.Errorf("tool %q is not enabled", name)
}

func (d *WorkspaceDriver) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	errs := validateToolArgs(req.Tool, req.Arguments)
	return &plugins.ValidateResponse{Valid: len(errs) == 0, Errors: errs}, nil
}

// matchesFilter reports whether def satisfies the given filter.
func matchesFilter(def plugins.ToolDefinition, f plugins.ListToolsFilter) bool {
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

// requiredStringArg returns an error string if the named argument is absent or not a string.
func requiredStringArg(args map[string]any, name string) string {
	v, ok := args[name]
	if !ok {
		return fmt.Sprintf("%q is required", name)
	}
	if _, ok := v.(string); !ok {
		return fmt.Sprintf("%q must be a string", name)
	}
	return ""
}

// validateToolArgs performs argument validation for each known tool.
func validateToolArgs(tool string, args map[string]any) []string {
	var errs []string
	add := func(e string) {
		if e != "" {
			errs = append(errs, e)
		}
	}
	switch tool {
	case "create", "read", "delete", "list":
		if tool != "list" {
			add(requiredStringArg(args, "path"))
		}
	case "write":
		add(requiredStringArg(args, "path"))
		add(requiredStringArg(args, "content"))
	case "exec":
		add(requiredStringArg(args, "command"))
	default:
		errs = append(errs, fmt.Sprintf("unknown tool %q", tool))
	}
	return errs
}

func (d *WorkspaceDriver) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	// Verify the tool is enabled
	enabled := false
	for _, t := range d.config.Tools {
		if t == req.Tool {
			enabled = true
			break
		}
	}
	if !enabled {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("tool '%s' is not enabled in workspace configuration", req.Tool),
			IsError: true,
		}, nil
	}

	switch req.Tool {
	case "create":
		return d.execCreate(req.Arguments)
	case "read":
		return d.execRead(req.Arguments)
	case "write":
		return d.execWrite(req.Arguments)
	case "delete":
		return d.execDelete(req.Arguments)
	case "list":
		return d.execList(req.Arguments)
	case "exec":
		return d.execCommand(ctx, req.Arguments)
	default:
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("unknown tool: %s", req.Tool),
			IsError: true,
		}, nil
	}
}

// resolvePath converts a path to an absolute path rooted at workspace home.
// Relative paths are joined to home; absolute paths and ~-prefixed paths are expanded as-is.
func (d *WorkspaceDriver) resolvePath(path string) (string, error) {
	if path == "" {
		return d.config.Home, nil
	}

	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(d.config.Home, path)
	}

	return filepath.Clean(path), nil
}

// validatePath checks that path is within the workspace home directory.
// For paths outside home, it checks the allowlist patterns.
func (d *WorkspaceDriver) validatePath(path string) error {
	home := d.config.Home
	if strings.HasPrefix(path, home+string(filepath.Separator)) || path == home {
		return nil
	}

	for _, pattern := range d.config.Allowlist {
		if strings.HasPrefix(pattern, "~") {
			userHome, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			pattern = filepath.Join(userHome, pattern[1:])
		}

		re, err := regexp.Compile(pattern)
		if err == nil {
			if re.MatchString(path) {
				return nil
			}
			continue
		}

		// Fall back to filepath.Match for glob-style patterns
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return nil
		}
	}

	return fmt.Errorf("path '%s' is outside workspace home and not permitted by allowlist", path)
}

// validateCommand checks the command against the allowlist when one is configured.
// If the allowlist is empty, any command is permitted.
func (d *WorkspaceDriver) validateCommand(command string) error {
	if len(d.config.Allowlist) == 0 {
		return nil
	}

	for _, pattern := range d.config.Allowlist {
		if strings.HasPrefix(pattern, "~") {
			userHome, err := os.UserHomeDir()
			if err != nil {
				continue
			}
			pattern = filepath.Join(userHome, pattern[1:])
		}

		re, err := regexp.Compile(pattern)
		if err == nil {
			if re.MatchString(command) {
				return nil
			}
			continue
		}

		matched, err := filepath.Match(pattern, command)
		if err == nil && matched {
			return nil
		}
	}

	return fmt.Errorf("command '%s' is not permitted by allowlist", command)
}

func (d *WorkspaceDriver) execCreate(args map[string]any) (*plugins.ExecuteResponse, error) {
	pathArg, ok := args["path"].(string)
	if !ok || pathArg == "" {
		return &plugins.ExecuteResponse{Result: "path is required", IsError: true}, nil
	}

	resolved, err := d.resolvePath(pathArg)
	if err != nil {
		return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
	}
	if err := d.validatePath(resolved); err != nil {
		return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
	}

	if _, err := os.Stat(resolved); err == nil {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("file '%s' already exists", pathArg),
			IsError: true,
		}, nil
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0755); err != nil {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("failed to create parent directories: %v", err),
			IsError: true,
		}, nil
	}

	content := ""
	if c, ok := args["content"].(string); ok {
		content = c
	}

	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("failed to create file: %v", err),
			IsError: true,
		}, nil
	}

	d.log.Debug("Created file", "path", resolved)
	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"path":    pathArg,
			"created": true,
		},
	}, nil
}

func (d *WorkspaceDriver) execRead(args map[string]any) (*plugins.ExecuteResponse, error) {
	pathArg, ok := args["path"].(string)
	if !ok || pathArg == "" {
		return &plugins.ExecuteResponse{Result: "path is required", IsError: true}, nil
	}

	resolved, err := d.resolvePath(pathArg)
	if err != nil {
		return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
	}
	if err := d.validatePath(resolved); err != nil {
		return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("failed to read file: %v", err),
			IsError: true,
		}, nil
	}

	d.log.Debug("Read file", "path", resolved, "bytes", len(data))
	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"path":    pathArg,
			"content": string(data),
			"size":    len(data),
		},
	}, nil
}

func (d *WorkspaceDriver) execWrite(args map[string]any) (*plugins.ExecuteResponse, error) {
	pathArg, ok := args["path"].(string)
	if !ok || pathArg == "" {
		return &plugins.ExecuteResponse{Result: "path is required", IsError: true}, nil
	}
	content, ok := args["content"].(string)
	if !ok {
		return &plugins.ExecuteResponse{Result: "content is required", IsError: true}, nil
	}

	resolved, err := d.resolvePath(pathArg)
	if err != nil {
		return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
	}
	if err := d.validatePath(resolved); err != nil {
		return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0755); err != nil {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("failed to create parent directories: %v", err),
			IsError: true,
		}, nil
	}

	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("failed to write file: %v", err),
			IsError: true,
		}, nil
	}

	d.log.Debug("Wrote file", "path", resolved, "bytes", len(content))
	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"path":    pathArg,
			"written": true,
			"size":    len(content),
		},
	}, nil
}

func (d *WorkspaceDriver) execDelete(args map[string]any) (*plugins.ExecuteResponse, error) {
	pathArg, ok := args["path"].(string)
	if !ok || pathArg == "" {
		return &plugins.ExecuteResponse{Result: "path is required", IsError: true}, nil
	}

	resolved, err := d.resolvePath(pathArg)
	if err != nil {
		return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
	}
	if err := d.validatePath(resolved); err != nil {
		return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
	}

	if resolved == d.config.Home {
		return &plugins.ExecuteResponse{Result: "cannot delete workspace home directory", IsError: true}, nil
	}

	if err := os.RemoveAll(resolved); err != nil {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("failed to delete: %v", err),
			IsError: true,
		}, nil
	}

	d.log.Debug("Deleted", "path", resolved)
	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"path":    pathArg,
			"deleted": true,
		},
	}, nil
}

func (d *WorkspaceDriver) execList(args map[string]any) (*plugins.ExecuteResponse, error) {
	pathArg, _ := args["path"].(string)

	resolved, err := d.resolvePath(pathArg)
	if err != nil {
		return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
	}
	if err := d.validatePath(resolved); err != nil {
		return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("failed to list directory: %v", err),
			IsError: true,
		}, nil
	}

	type entry struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size,omitempty"`
	}

	files := make([]entry, 0, len(entries))
	for _, e := range entries {
		fe := entry{Name: e.Name(), IsDir: e.IsDir()}
		if !e.IsDir() {
			if info, err := e.Info(); err == nil {
				fe.Size = info.Size()
			}
		}
		files = append(files, fe)
	}

	d.log.Debug("Listed directory", "path", resolved, "count", len(files))
	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"path":    pathArg,
			"entries": files,
			"count":   len(files),
		},
	}, nil
}

func (d *WorkspaceDriver) execCommand(ctx context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return &plugins.ExecuteResponse{Result: "command is required", IsError: true}, nil
	}

	if err := d.validateCommand(command); err != nil {
		return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
	}

	var cmdArgs []string
	if argsVal, ok := args["args"]; ok {
		switch v := argsVal.(type) {
		case []any:
			for _, a := range v {
				if s, ok := a.(string); ok {
					cmdArgs = append(cmdArgs, s)
				}
			}
		case string:
			if v != "" {
				cmdArgs = strings.Fields(v)
			}
		}
	}

	workdir := d.config.Home
	if wdArg, ok := args["workdir"].(string); ok && wdArg != "" {
		resolved, err := d.resolvePath(wdArg)
		if err != nil {
			return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
		}
		if err := d.validatePath(resolved); err != nil {
			return &plugins.ExecuteResponse{Result: err.Error(), IsError: true}, nil
		}
		workdir = resolved
	}

	cmd := exec.CommandContext(ctx, command, cmdArgs...)
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	d.log.Debug("Executing command", "command", command, "args", cmdArgs, "workdir", workdir)

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return &plugins.ExecuteResponse{
				Result:  fmt.Sprintf("failed to start command: %v", err),
				IsError: true,
			}, nil
		}
	}

	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"command":   command,
			"args":      cmdArgs,
			"stdout":    stdout.String(),
			"stderr":    stderr.String(),
			"exit_code": exitCode,
		},
		IsError: exitCode != 0,
	}, nil
}
