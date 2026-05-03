package resource

import (
	"context"
	"fmt"
	"time"

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
		path := s.resolvePath(ctx, req.Arguments)
		tags := argStringSlice(req.Arguments, "tags")
		meta, _ := req.Arguments["metadata"].(map[string]any)
		res, err := s.Store(ctx, path, content, tags, meta)
		if err != nil {
			return nil, err
		}
		return &plugins.ExecuteResponse{Result: res}, nil

	case "recall":
		path := s.resolvePath(ctx, req.Arguments)
		q, err := recallQueryFromArgs(req.Arguments, path)
		if err != nil {
			return nil, err
		}
		res, err := s.Recall(ctx, q)
		if err != nil {
			return nil, err
		}
		return &plugins.ExecuteResponse{Result: res}, nil

	case "forget":
		id, ok := argString(req.Arguments, "id")
		if !ok {
			return nil, fmt.Errorf("missing argument %q", "id")
		}
		path := s.resolvePath(ctx, req.Arguments)
		if err := s.Forget(ctx, path, id); err != nil {
			return nil, err
		}
		return &plugins.ExecuteResponse{Result: map[string]any{"id": id, "path": path}}, nil
	}

	return nil, fmt.Errorf("unknown tool execution: %s (%s)", req.Tool, req.CallID)
}

func (s *ResourceService) resolvePath(ctx context.Context, args map[string]any) string {
	if v, ok := argString(args, "path"); ok {
		return v
	}
	if id := sessionctx.From(ctx); id != "" {
		return "/sessions/" + id
	}
	if s.config.DefaultPath != "" {
		return s.config.DefaultPath
	}
	return "/global"
}

// recallQueryFromArgs parses the flat tool argument map into a RecallQuery.
// defaultPath is used when args does not specify "path".
func recallQueryFromArgs(args map[string]any, defaultPath string) (plugins.RecallQuery, error) {
	path, ok := argString(args, "path")
	if !ok {
		path = defaultPath
	}

	q := plugins.RecallQuery{
		Path:  path,
		Query: argStringOptional(args, "query"),
		Tags:  argStringSlice(args, "tags"),
		Limit: argInt(args, "limit", 5),
	}

	// filter: [{key, op, value}, ...]
	if raw, ok := args["filter"].([]any); ok {
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			key, _ := m["key"].(string)
			op, _ := m["op"].(string)
			val := m["value"]
			if key != "" && op != "" {
				q.Filter = append(q.Filter, plugins.FilterPredicate{
					Key:   key,
					Op:    plugins.FilterOp(op),
					Value: val,
				})
			}
		}
	}

	if v, ok := argString(args, "created_after"); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.CreatedAfter = t
		}
	}
	if v, ok := argString(args, "created_before"); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.CreatedBefore = t
		}
	}

	return q, nil
}

func argString(args map[string]any, key string) (string, bool) {
	v, ok := args[key].(string)
	return v, ok && v != ""
}

func argStringOptional(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argStringSlice(args map[string]any, key string) []string {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
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
