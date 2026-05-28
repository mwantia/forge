package pipeline

import (
	"context"
	"fmt"

	"github.com/mwantia/forge-sdk/pkg/plugins"
	appsession "github.com/mwantia/forge/internal/application/session"
)

func (s *PipelineService) ExecuteTool(ctx context.Context, request plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	args := request.Args.AsMap()
	switch request.Tool {
	case "commit_session":
		sessionID, err := appsession.ResolveSessionArg(ctx, args, "session_id")
		if err != nil {
			return nil, err
		}

		content, ok := appsession.ArgString(args, "content")
		if !ok || content == "" {
			return nil, fmt.Errorf("missing argument %q", "content")
		}

		ref, _ := appsession.ArgString(args, "ref")
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
