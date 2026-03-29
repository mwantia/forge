package tools

import (
	"context"
	"fmt"

	"github.com/mwantia/forge/pkg/plugins"
	proto "github.com/mwantia/forge/pkg/plugins/grpc/tools/proto"
	"google.golang.org/grpc"
)

// Client implements plugins.ToolsPlugin over gRPC.
type Client struct {
	client proto.ToolsServiceClient
}

func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{client: proto.NewToolsServiceClient(conn)}
}

func (c *Client) GetLifecycle() plugins.Lifecycle { return nil }

func (c *Client) ListTools(ctx context.Context, filter plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	resp, err := c.client.ListTools(ctx, &proto.ListToolsRequest{
		Tags:       filter.Tags,
		Deprecated: filter.Deprecated,
		Prefix:     filter.Prefix,
	})
	if err != nil {
		return nil, err
	}

	result := &plugins.ListToolsResponse{}
	for _, t := range resp.Tools {
		result.Tools = append(result.Tools, protoToToolDefinition(t))
	}
	return result, nil
}

func (c *Client) GetTool(ctx context.Context, name string) (*plugins.ToolDefinition, error) {
	resp, err := c.client.GetTool(ctx, &proto.GetToolRequest{Name: name})
	if err != nil {
		return nil, err
	}
	def := protoToToolDefinition(resp.Tool)
	return &def, nil
}

func (c *Client) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	argsStruct, err := toStruct(req.Arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to encode arguments: %w", err)
	}

	resp, err := c.client.Execute(ctx, &proto.ExecuteRequest{
		Tool:      req.Tool,
		Arguments: argsStruct,
		CallId:    req.CallID,
	})
	if err != nil {
		return nil, err
	}

	var result any
	if resp.Result != nil {
		result = resp.Result.AsInterface()
	}

	var metadata map[string]any
	if resp.Metadata != nil {
		metadata = resp.Metadata.AsMap()
	}

	return &plugins.ExecuteResponse{
		Result:   result,
		IsError:  resp.IsError,
		Metadata: metadata,
	}, nil
}

func (c *Client) ExecuteStream(ctx context.Context, req plugins.ExecuteRequest) (<-chan plugins.ExecuteChunk, error) {
	argsStruct, err := toStruct(req.Arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to encode arguments: %w", err)
	}

	stream, err := c.client.ExecuteStream(ctx, &proto.ExecuteRequest{
		Tool:      req.Tool,
		Arguments: argsStruct,
		CallId:    req.CallID,
	})
	if err != nil {
		return nil, err
	}

	ch := make(chan plugins.ExecuteChunk)
	go func() {
		defer close(ch)
		for {
			chunk, err := stream.Recv()
			if err != nil {
				if !isEOF(err) {
					ch <- plugins.ExecuteChunk{IsError: true, Delta: err.Error(), Done: true}
				}
				return
			}
			var delta any
			if chunk.Delta != nil {
				delta = chunk.Delta.AsInterface()
			}
			ch <- plugins.ExecuteChunk{
				CallID:  chunk.CallId,
				Delta:   delta,
				Done:    chunk.Done,
				IsError: chunk.IsError,
			}
			if chunk.Done {
				return
			}
		}
	}()

	return ch, nil
}

func (c *Client) Cancel(ctx context.Context, callID string) error {
	resp, err := c.client.Cancel(ctx, &proto.CancelRequest{CallId: callID})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return fmt.Errorf("cancel not acknowledged for call_id %q", callID)
	}
	return nil
}

func (c *Client) Validate(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	argsStruct, err := toStruct(req.Arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to encode arguments: %w", err)
	}

	resp, err := c.client.Validate(ctx, &proto.ValidateRequest{
		Tool:      req.Tool,
		Arguments: argsStruct,
	})
	if err != nil {
		return nil, err
	}

	return &plugins.ValidateResponse{
		Valid:  resp.Valid,
		Errors: resp.Errors,
	}, nil
}

// protoToToolDefinition converts a ToolDefProto to plugins.ToolDefinition.
func protoToToolDefinition(t *proto.ToolDefinitionProto) plugins.ToolDefinition {
	if t == nil {
		return plugins.ToolDefinition{}
	}
	var params map[string]any
	if t.Parameters != nil {
		params = t.Parameters.AsMap()
	}
	def := plugins.ToolDefinition{
		Name:               t.Name,
		Description:        t.Description,
		Parameters:         params,
		Tags:               t.Tags,
		Version:            t.Version,
		Deprecated:         t.Deprecated,
		DeprecationMessage: t.DeprecationMessage,
	}
	if t.Annotations != nil {
		def.Annotations = plugins.ToolAnnotations{
			ReadOnly:             t.Annotations.ReadOnly,
			Destructive:          t.Annotations.Destructive,
			Idempotent:           t.Annotations.Idempotent,
			RequiresConfirmation: t.Annotations.RequiresConfirmation,
			CostHint:             plugins.ToolCostHint(t.Annotations.CostHint),
		}
	}
	return def
}
