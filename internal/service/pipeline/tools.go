package pipeline

import (
	"context"
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge/internal/service/session"
)

func (s *PipelineService) ExecuteTool(ctx context.Context, request plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	switch request.Tool {
	case "commit_session":
		sessionID, err := session.ResolveSessionArg(ctx, request.Arguments, "session_id")
		if err != nil {
			return nil, err
		}

		content, ok := session.ArgString(request.Arguments, "content")
		if !ok || content == "" {
			return nil, fmt.Errorf("missing argument %q", "content")
		}

		ref, _ := session.ArgString(request.Arguments, "ref")
		response, err := s.CommitSync(ctx, sessionID, ref, content)
		if err != nil {
			return nil, fmt.Errorf("commit_session: %w", err)
		}

		return &plugins.ExecuteResponse{
			Result: response,
		}, nil
	}

	return nil, fmt.Errorf("unknown tool execution: %s (%s)", request.Tool, request.CallID)
}
