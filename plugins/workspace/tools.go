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

// toolDefinitions maps tool names to their JSON Schema definitions.
var toolDefinitions = map[string]plugins.ToolDefinition{
	"create": {
		Name:        "create",
		Description: "Create a new file with optional content",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path relative to workspace home",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Initial file content (optional, defaults to empty)",
				},
			},
			"required": []string{"path"},
		},
	},
	"read": {
		Name:        "read",
		Description: "Read the contents of a file",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path relative to workspace home",
				},
			},
			"required": []string{"path"},
		},
	},
	"write": {
		Name:        "write",
		Description: "Write content to a file, creating it if it does not exist",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path relative to workspace home",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			"required": []string{"path", "content"},
		},
	},
	"delete": {
		Name:        "delete",
		Description: "Delete a file or directory (recursive)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File or directory path relative to workspace home",
				},
			},
			"required": []string{"path"},
		},
	},
	"list": {
		Name:        "list",
		Description: "List the contents of a directory",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path relative to workspace home (defaults to home directory)",
				},
			},
		},
	},
	"exec": {
		Name:        "exec",
		Description: "Execute a command in the workspace directory",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Command to execute",
				},
				"args": map[string]any{
					"type":        "array",
					"description": "Arguments to pass to the command",
					"items":       map[string]any{"type": "string"},
				},
				"workdir": map[string]any{
					"type":        "string",
					"description": "Working directory for the command (defaults to workspace home)",
				},
			},
			"required": []string{"command"},
		},
	},
}

func (d *WorkspaceDriver) GetLifecycle() plugins.Lifecycle {
	return d
}

func (d *WorkspaceDriver) List(ctx context.Context) (*plugins.ListToolsResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	tools := make([]plugins.ToolDefinition, 0, len(d.config.Tools))
	for _, name := range d.config.Tools {
		if def, ok := toolDefinitions[name]; ok {
			tools = append(tools, def)
		} else {
			d.log.Warn("Unknown tool in config, skipping", "tool", name)
		}
	}

	return &plugins.ListToolsResponse{Tools: tools}, nil
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
