package resource

import (
	"context"
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/sessionctx"
)

func (s *ResourceService) ExecuteTool(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	switch req.Tool {
	case "store":
		content, ok := argString(req.Arguments, "content")
		if !ok {
			return nil, fmt.Errorf("missing argument %q", "content")
		}
		ns := s.resolveNamespace(ctx, req.Arguments)
		meta, _ := req.Arguments["metadata"].(map[string]any)
		res, err := s.Store(ctx, ns, content, meta)
		if err != nil {
			return nil, err
		}
		return &plugins.ExecuteResponse{Result: res}, nil

	case "recall":
		query, ok := argString(req.Arguments, "query")
		if !ok {
			return nil, fmt.Errorf("missing argument %q", "query")
		}
		ns := s.resolveNamespace(ctx, req.Arguments)
		limit := argInt(req.Arguments, "limit", 5)
		filter, _ := req.Arguments["filter"].(map[string]any)
		res, err := s.Recall(ctx, ns, query, limit, filter)
		if err != nil {
			return nil, err
		}
		return &plugins.ExecuteResponse{Result: res}, nil

	case "forget":
		id, ok := argString(req.Arguments, "id")
		if !ok {
			return nil, fmt.Errorf("missing argument %q", "id")
		}
		ns := s.resolveNamespace(ctx, req.Arguments)
		if err := s.Forget(ctx, ns, id); err != nil {
			return nil, err
		}
		return &plugins.ExecuteResponse{Result: map[string]any{"id": id, "namespace": ns}}, nil
	}

	return nil, fmt.Errorf("unknown tool execution: %s (%s)", req.Tool, req.CallID)
}

func (s *ResourceService) resolveNamespace(ctx context.Context, args map[string]any) string {
	if v, ok := argString(args, "namespace"); ok {
		return v
	}
	if id := sessionctx.From(ctx); id != "" {
		return id
	}
	if s.config.DefaultNamespace != "" {
		return s.config.DefaultNamespace
	}
	return "global"
}

func argString(args map[string]any, key string) (string, bool) {
	v, ok := args[key].(string)
	return v, ok && v != ""
}

func argInt(args map[string]any, key string, def int) int {
	switch v := args[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return def
}
