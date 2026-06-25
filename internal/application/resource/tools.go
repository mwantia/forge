package resource

import (
	"context"
	"fmt"
	"time"

	plugins "github.com/mwantia/forge-sdk/pkg/plugin"
	domresource "github.com/mwantia/forge/internal/domain/resource"
	domsession "github.com/mwantia/forge/internal/domain/session"
)

func (s *ResourceService) ExecuteTool(ctx context.Context, req plugins.ExecuteToolRequest) (*plugins.ExecuteToolResponse, error) {
	args := req.Args.AsMap()
	switch req.Tool {
	case "store":
		content, ok := argString(args, "content")
		if !ok {
			return nil, fmt.Errorf("missing argument %q", "content")
		}
		meta := domresource.ResourceMeta{
			Name:        argStringOptional(args, "name"),
			Type:        argStringOptional(args, "type"),
			Description: argStringOptional(args, "description"),
			Tags:        argStringSlice(args, "tags"),
			Session:     domsession.CallerSessionID(ctx),
		}
		if extra, ok := args["extra"].(map[string]any); ok {
			meta.Extra = extra
		}
		res, err := s.Store(ctx, content, argStringOptional(args, "commit_message"), meta)
		if err != nil {
			return nil, err
		}
		return plugins.ExecuteSuccess(map[string]any{
			"id":         res.ID,
			"name":       res.Meta.Name,
			"type":       res.Meta.Type,
			"created_at": res.Meta.CreatedAt,
		}), nil

	case "commit":
		id, ok := argString(args, "id")
		if !ok {
			return nil, fmt.Errorf("missing argument %q", "id")
		}
		content, ok := argString(args, "content")
		if !ok {
			return nil, fmt.Errorf("missing argument %q", "content")
		}
		res, err := s.Commit(ctx, id, content, argStringOptional(args, "commit_message"))
		if err != nil {
			return nil, err
		}
		return plugins.ExecuteSuccess(map[string]any{
			"id":         res.ID,
			"name":       res.Meta.Name,
			"type":       res.Meta.Type,
			"updated_at": res.Meta.UpdatedAt,
		}), nil

	case "recall":
		q := domresource.RecallQuery{
			Query: argStringOptional(args, "query"),
			Tags:  argStringSlice(args, "tags"),
			Limit: argInt(args, "limit", 5),
		}
		// Convenience shortcuts that translate to filter predicates.
		if typ := argStringOptional(args, "type"); typ != "" {
			q.Filter = append(q.Filter, domresource.FilterPredicate{
				Key: "type", Op: domresource.FilterOpEq, Value: typ,
			})
		}
		if argStringOptional(args, "scope") == "session" {
			if sid := domsession.CallerSessionID(ctx); sid != "" {
				q.Filter = append(q.Filter, domresource.FilterPredicate{
					Key: "session", Op: domresource.FilterOpEq, Value: sid,
				})
			}
		}
		// Additional typed filter predicates from the LLM.
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
					q.Filter = append(q.Filter, domresource.FilterPredicate{
						Key: key, Op: domresource.FilterOp(op), Value: val,
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
		res, err := s.Recall(ctx, q)
		if err != nil {
			return nil, err
		}
		return plugins.ExecuteSuccess(res), nil

	case "forget":
		id, ok := argString(args, "id")
		if !ok {
			return nil, fmt.Errorf("missing argument %q", "id")
		}
		if err := s.Forget(ctx, id); err != nil {
			return nil, err
		}
		return plugins.ExecuteSuccess(map[string]any{"id": id}), nil
	}

	return nil, fmt.Errorf("unknown tool execution: %s (%s)", req.Tool, req.CallID)
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
