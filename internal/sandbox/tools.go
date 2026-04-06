package sandbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mwantia/forge-sdk/pkg/plugins"
)

// SandboxToolsPlugin is a built-in ToolsPlugin that exposes sandbox management
// tools to the LLM agent under the "agent__sandbox_*" namespace.
// It is created per-dispatch and bound to the sandbox manager and session ID.
type SandboxToolsPlugin struct {
	plugins.UnimplementedToolsPlugin
	Manager   *SandboxManager
	SessionID string
}

func (p *SandboxToolsPlugin) GetLifecycle() plugins.Lifecycle { return nil }

func (p *SandboxToolsPlugin) ListTools(_ context.Context, filter plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	tools := make([]plugins.ToolDefinition, 0)
	for _, def := range toolDefinitions {
		if plugins.MatchesToolsFilter(def, filter) {
			tools = append(tools, def)
		}
	}

	return &plugins.ListToolsResponse{
		Tools: tools,
	}, nil
}

func (p *SandboxToolsPlugin) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	def, ok := toolDefinitions[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}

	return &def, nil
}

func (p *SandboxToolsPlugin) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	def, ok := toolDefinitions[req.Tool]
	if !ok {
		return &plugins.ValidateResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("unknown tool %q", req.Tool)},
		}, nil
	}
	return plugins.ValidateAgainstDefinition(def, req), nil
}

func (p *SandboxToolsPlugin) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	switch req.Tool {
	case "sandbox_create":
		return p.execCreate(ctx, req.Arguments)
	case "sandbox_destroy":
		return p.execDestroy(ctx, req.Arguments)
	case "sandbox_list":
		return p.execList(ctx)
	case "sandbox_exec":
		return p.execExec(ctx, req.Arguments)
	case "sandbox_copy_in":
		return p.execCopyIn(ctx, req.Arguments)
	case "sandbox_copy_out":
		return p.execCopyOut(ctx, req.Arguments)
	case "sandbox_stat":
		return p.execStat(ctx, req.Arguments)
	case "sandbox_read":
		return p.execRead(ctx, req.Arguments)
	default:
		return nil, fmt.Errorf("unknown sandbox tool %q", req.Tool)
	}
}

func (p *SandboxToolsPlugin) execCreate(ctx context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	opts := CreateOptions{SessionID: p.SessionID}
	if v, ok := args["name"].(string); ok {
		opts.Name = v
	}
	if v, ok := args["driver"].(string); ok {
		opts.IsolationDriver = v
	}
	if v, ok := args["work_dir"].(string); ok {
		opts.Spec.WorkDir = v
	}

	sb, err := p.Manager.Create(ctx, opts)
	if err != nil {
		return &plugins.ExecuteResponse{IsError: true, Result: err.Error()}, nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{
		"id":     sb.ID,
		"name":   sb.Name,
		"status": string(sb.Status),
	}}, nil
}

func (p *SandboxToolsPlugin) execDestroy(ctx context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	id, _ := args["id"].(string)
	if err := p.Manager.Delete(ctx, id); err != nil {
		return &plugins.ExecuteResponse{IsError: true, Result: err.Error()}, nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"destroyed": true, "id": id}}, nil
}

func (p *SandboxToolsPlugin) execList(_ context.Context) (*plugins.ExecuteResponse, error) {
	sbs, err := p.Manager.List(ListOptions{SessionID: p.SessionID})
	if err != nil {
		return &plugins.ExecuteResponse{IsError: true, Result: err.Error()}, nil
	}
	items := make([]map[string]any, 0, len(sbs))
	for _, sb := range sbs {
		items = append(items, map[string]any{
			"id":     sb.ID,
			"name":   sb.Name,
			"status": string(sb.Status),
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"sandboxes": items}}, nil
}

func (p *SandboxToolsPlugin) execExec(ctx context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	id, _ := args["id"].(string)
	command, _ := args["command"].(string)

	var execArgs []string
	if raw, ok := args["args"].([]any); ok {
		for _, a := range raw {
			if s, ok := a.(string); ok {
				execArgs = append(execArgs, s)
			}
		}
	}

	timeout := 30
	if v, ok := args["timeout_seconds"].(float64); ok && v > 0 {
		timeout = int(v)
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	ch, err := p.Manager.Execute(execCtx, id, pluginsExecReq(id, command, execArgs, timeout))
	if err != nil {
		return &plugins.ExecuteResponse{IsError: true, Result: err.Error()}, nil
	}

	var stdout, stderr strings.Builder
	exitCode := 0
	for chunk := range ch {
		if chunk.IsError {
			return &plugins.ExecuteResponse{IsError: true, Result: chunk.Data}, nil
		}
		switch chunk.Stream {
		case "stdout":
			stdout.WriteString(chunk.Data)
		case "stderr":
			stderr.WriteString(chunk.Data)
		}
		if chunk.Done {
			exitCode = chunk.ExitCode
		}
	}

	return &plugins.ExecuteResponse{Result: map[string]any{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": exitCode,
	}}, nil
}

func (p *SandboxToolsPlugin) execCopyIn(ctx context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	id, _ := args["id"].(string)
	hostSrc, _ := args["host_src"].(string)
	sandboxDst, _ := args["sandbox_dst"].(string)
	if err := p.Manager.CopyIn(ctx, id, hostSrc, sandboxDst); err != nil {
		return &plugins.ExecuteResponse{IsError: true, Result: err.Error()}, nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"copied": true}}, nil
}

func (p *SandboxToolsPlugin) execCopyOut(ctx context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	id, _ := args["id"].(string)
	sandboxSrc, _ := args["sandbox_src"].(string)
	hostDst, _ := args["host_dst"].(string)
	if err := p.Manager.CopyOut(ctx, id, sandboxSrc, hostDst); err != nil {
		return &plugins.ExecuteResponse{IsError: true, Result: err.Error()}, nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"copied": true}}, nil
}

func (p *SandboxToolsPlugin) execStat(ctx context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	id, _ := args["id"].(string)
	path, _ := args["path"].(string)
	result, err := p.Manager.Stat(ctx, id, path)
	if err != nil {
		return &plugins.ExecuteResponse{IsError: true, Result: err.Error()}, nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{
		"path":     result.Path,
		"exists":   result.Exists,
		"is_dir":   result.IsDir,
		"size":     result.Size,
		"mode":     result.Mode,
		"mod_time": result.ModTime,
	}}, nil
}

func (p *SandboxToolsPlugin) execRead(ctx context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	id, _ := args["id"].(string)
	path, _ := args["path"].(string)
	data, err := p.Manager.ReadFile(ctx, id, path)
	if err != nil {
		return &plugins.ExecuteResponse{IsError: true, Result: err.Error()}, nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{
		"path":    path,
		"content": string(data),
	}}, nil
}

func pluginsExecReq(id, command string, args []string, timeout int) plugins.SandboxExecRequest {
	return plugins.SandboxExecRequest{
		SandboxID:      id,
		Command:        command,
		Args:           args,
		TimeoutSeconds: timeout,
	}
}
