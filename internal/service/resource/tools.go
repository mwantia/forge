package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/sessionctx"
)

func (s *ResourceService) ExecuteTool(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	args := req.Args.AsMap()
	switch req.Tool {
	case "store":
		content, ok := argString(args, "content")
		if !ok {
			return nil, fmt.Errorf("missing argument %q", "content")
		}
		path := s.resolvePathFromType(ctx, args, false)
		name := argStringOptional(args, "name")
		tags := argStringSlice(args, "tags")
		meta, _ := args["metadata"].(map[string]any)
		res, err := s.Store(ctx, path, name, content, tags, meta)
		if err != nil {
			return nil, err
		}
		return &plugins.ExecuteResponse{Result: res}, nil

	case "recall":
		path := s.resolvePathFromType(ctx, args, true)
		q, err := recallQueryFromArgs(args, path)
		if err != nil {
			return nil, err
		}
		res, err := s.Recall(ctx, q)
		if err != nil {
			return nil, err
		}
		return &plugins.ExecuteResponse{Result: res}, nil

	case "forget":
		name, ok := argString(args, "name")
		if !ok {
			return nil, fmt.Errorf("missing argument %q", "name")
		}
		path := s.resolvePathFromType(ctx, args, false)
		if err := s.Forget(ctx, path, name); err != nil {
			return nil, err
		}
		return &plugins.ExecuteResponse{Result: map[string]any{"name": name, "path": path}}, nil
	}

	return nil, fmt.Errorf("unknown tool execution: %s (%s)", req.Tool, req.CallID)
}

// resolvePathFromType maps the "type" argument to a canonical storage path.
// allowAny enables the "any" glob type (recall only); store/forget pass false.
func (s *ResourceService) resolvePathFromType(ctx context.Context, args map[string]any, allowAny bool) string {
	typ := argStringOptional(args, "type")
	sessionID := sessionctx.From(ctx)

	switch typ {
	case "memory":
		if sessionID != "" {
			return "/forge/sessions/" + sessionID + "/memories"
		}
		return "/forge/global/memories"
	case "reference":
		if sessionID != "" {
			return "/forge/sessions/" + sessionID + "/references"
		}
		return "/forge/global/references"
	case "online":
		if sessionID != "" {
			return "/forge/sessions/" + sessionID + "/online"
		}
		return "/forge/global/online"
	case "global":
		return "/forge/global"
	default:
		// "any" or missing type — glob covers all forge-managed paths.
		if allowAny {
			return "/forge/**"
		}
		if sessionID != "" {
			return "/forge/sessions/" + sessionID
		}
		return "/forge"
	}
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
