package plugins

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-plugin"
	"github.com/mwantia/forge/pkg/plugins/proto"
	"google.golang.org/grpc"
)

// DriverPlugin is the hashicorp/go-plugin wrapper for the Driver interface.
type DriverPlugin struct {
	plugin.Plugin
	Impl Driver
}

func (p *DriverPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Register the driver service
	proto.RegisterDriverServiceServer(s, &driverGRPCServer{
		Impl:   p.Impl,
		broker: broker,
	})

	// Register plugin-specific services that delegate to the driver's plugins
	proto.RegisterProviderServiceServer(s, &providerGRPCServer{Impl: p.Impl})
	proto.RegisterMemoryServiceServer(s, &memoryGRPCServer{Impl: p.Impl})
	proto.RegisterChannelServiceServer(s, &channelGRPCServer{Impl: p.Impl})
	proto.RegisterToolsServiceServer(s, &toolsGRPCServer{Impl: p.Impl})

	return nil
}

func (p *DriverPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return &driverGRPCClient{
		client: proto.NewDriverServiceClient(c),
		broker: broker,
		conn:   c,
	}, nil
}

// driverGRPCServer implements DriverServiceServer for gRPC.
type driverGRPCServer struct {
	proto.UnimplementedDriverServiceServer
	Impl   Driver
	broker *plugin.GRPCBroker
}

func (s *driverGRPCServer) Name(ctx context.Context, req *proto.NameRequest) (*proto.NameResponse, error) {
	return &proto.NameResponse{Name: s.Impl.Name()}, nil
}

func (s *driverGRPCServer) ProbePlugin(ctx context.Context, req *proto.ProbeRequest) (*proto.ProbeResponse, error) {
	ok, err := s.Impl.ProbePlugin(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.ProbeResponse{Ok: ok}, nil
}

func (s *driverGRPCServer) GetCapabilities(ctx context.Context, req *proto.CapabilitiesRequest) (*proto.CapabilitiesResponse, error) {
	caps, err := s.Impl.GetCapabilities(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.CapabilitiesResponse{Capabilities: caps}, nil
}

func (s *driverGRPCServer) OpenDriver(ctx context.Context, req *proto.OpenRequest) (*proto.OpenResponse, error) {
	if err := s.Impl.OpenDriver(ctx); err != nil {
		return nil, err
	}
	return &proto.OpenResponse{}, nil
}

func (s *driverGRPCServer) CloseDriver(ctx context.Context, req *proto.CloseRequest) (*proto.CloseResponse, error) {
	if err := s.Impl.CloseDriver(ctx); err != nil {
		return nil, err
	}
	return &proto.CloseResponse{}, nil
}

func (s *driverGRPCServer) ConfigDriver(ctx context.Context, req *proto.ConfigRequest) (*proto.ConfigResponse, error) {
	config := PluginConfig{
		ConfigMap: make(map[string]any),
	}
	for k, v := range req.Config {
		config.ConfigMap[k] = v
	}
	if err := s.Impl.ConfigDriver(ctx, config); err != nil {
		return nil, err
	}
	return &proto.ConfigResponse{}, nil
}

func (s *driverGRPCServer) GetProviderPlugin(ctx context.Context, req *proto.GetPluginRequest) (*proto.GetPluginResponse, error) {
	plugin, err := s.Impl.GetProviderPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return &proto.GetPluginResponse{Info: nil}, nil
	}
	return &proto.GetPluginResponse{Info: plugin.GetPluginInfo()}, nil
}

func (s *driverGRPCServer) GetMemoryPlugin(ctx context.Context, req *proto.GetPluginRequest) (*proto.GetPluginResponse, error) {
	plugin, err := s.Impl.GetMemoryPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return &proto.GetPluginResponse{Info: nil}, nil
	}
	return &proto.GetPluginResponse{Info: plugin.GetPluginInfo()}, nil
}

func (s *driverGRPCServer) GetChannelPlugin(ctx context.Context, req *proto.GetPluginRequest) (*proto.GetPluginResponse, error) {
	plugin, err := s.Impl.GetChannelPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return &proto.GetPluginResponse{Info: nil}, nil
	}
	return &proto.GetPluginResponse{Info: plugin.GetPluginInfo()}, nil
}

func (s *driverGRPCServer) GetToolsPlugin(ctx context.Context, req *proto.GetPluginRequest) (*proto.GetPluginResponse, error) {
	plugin, err := s.Impl.GetToolsPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return &proto.GetPluginResponse{Info: nil}, nil
	}
	return &proto.GetPluginResponse{Info: plugin.GetPluginInfo()}, nil
}

// driverGRPCClient implements Driver for gRPC client.
type driverGRPCClient struct {
	client proto.DriverServiceClient
	broker *plugin.GRPCBroker
	conn   *grpc.ClientConn
}

func (c *driverGRPCClient) Name() string {
	resp, err := c.client.Name(context.Background(), &proto.NameRequest{})
	if err != nil {
		return ""
	}
	return resp.Name
}

func (c *driverGRPCClient) ProbePlugin(ctx context.Context) (bool, error) {
	resp, err := c.client.ProbePlugin(ctx, &proto.ProbeRequest{})
	if err != nil {
		return false, err
	}
	return resp.Ok, nil
}

func (c *driverGRPCClient) GetCapabilities(ctx context.Context) (*proto.DriverCapabilities, error) {
	resp, err := c.client.GetCapabilities(ctx, &proto.CapabilitiesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Capabilities, nil
}

func (c *driverGRPCClient) OpenDriver(ctx context.Context) error {
	_, err := c.client.OpenDriver(ctx, &proto.OpenRequest{})
	return err
}

func (c *driverGRPCClient) CloseDriver(ctx context.Context) error {
	_, err := c.client.CloseDriver(ctx, &proto.CloseRequest{})
	return err
}

func (c *driverGRPCClient) ConfigDriver(ctx context.Context, config PluginConfig) error {
	req := &proto.ConfigRequest{
		Config: make(map[string]string),
	}
	for k, v := range config.ConfigMap {
		switch val := v.(type) {
		case string:
			req.Config[k] = val
		case bool:
			if val {
				req.Config[k] = "true"
			} else {
				req.Config[k] = "false"
			}
		case int, int64:
			req.Config[k] = fmt.Sprintf("%d", val)
		case float64:
			req.Config[k] = fmt.Sprintf("%f", val)
		default:
			req.Config[k] = fmt.Sprintf("%v", val)
		}
	}
	_, err := c.client.ConfigDriver(ctx, req)
	return err
}

func (c *driverGRPCClient) GetProviderPlugin(ctx context.Context) (ProviderPlugin, error) {
	resp, err := c.client.GetProviderPlugin(ctx, &proto.GetPluginRequest{})
	if err != nil {
		return nil, err
	}
	if resp.Info == nil || resp.Info.Name == "" {
		return nil, nil
	}
	return &providerPluginWrapper{info: resp.Info, client: proto.NewProviderServiceClient(c.conn)}, nil
}

func (c *driverGRPCClient) GetMemoryPlugin(ctx context.Context) (MemoryPlugin, error) {
	resp, err := c.client.GetMemoryPlugin(ctx, &proto.GetPluginRequest{})
	if err != nil {
		return nil, err
	}
	if resp.Info == nil || resp.Info.Name == "" {
		return nil, nil
	}
	return &memoryPluginWrapper{info: resp.Info, client: proto.NewMemoryServiceClient(c.conn)}, nil
}

func (c *driverGRPCClient) GetChannelPlugin(ctx context.Context) (ChannelPlugin, error) {
	resp, err := c.client.GetChannelPlugin(ctx, &proto.GetPluginRequest{})
	if err != nil {
		return nil, err
	}
	if resp.Info == nil || resp.Info.Name == "" {
		return nil, nil
	}
	return &channelPluginWrapper{info: resp.Info, client: proto.NewChannelServiceClient(c.conn)}, nil
}

func (c *driverGRPCClient) GetToolsPlugin(ctx context.Context) (ToolsPlugin, error) {
	resp, err := c.client.GetToolsPlugin(ctx, &proto.GetPluginRequest{})
	if err != nil {
		return nil, err
	}
	if resp.Info == nil || resp.Info.Name == "" {
		return nil, nil
	}
	return &toolsPluginWrapper{info: resp.Info, client: proto.NewToolsServiceClient(c.conn)}, nil
}

// Plugin wrappers for client-side implementations.

type providerPluginWrapper struct {
	info   *proto.PluginInfo
	client proto.ProviderServiceClient
}

func (w *providerPluginWrapper) GetLifecycle() Lifecycle          { return nil }
func (w *providerPluginWrapper) GetPluginInfo() *proto.PluginInfo { return w.info }
func (w *providerPluginWrapper) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	protoReq := &proto.GenerateReq{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   int32(req.MaxTokens),
	}
	for _, m := range req.Messages {
		protoReq.Messages = append(protoReq.Messages, &proto.MessageProto{Role: m.Role, Content: m.Content})
	}
	for _, t := range req.Tools {
		params := make(map[string]string)
		for k, v := range t.Parameters {
			params[k] = fmt.Sprintf("%v", v)
		}
		protoReq.Tools = append(protoReq.Tools, &proto.ToolProto{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}

	resp, err := w.client.Generate(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	result := &GenerateResponse{
		ID:      resp.Id,
		Content: resp.Content,
		Role:    resp.Role,
		Model:   resp.Model,
	}
	if resp.Usage != nil {
		result.Usage = &Usage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
		}
	}
	return result, nil
}

type memoryPluginWrapper struct {
	info   *proto.PluginInfo
	client proto.MemoryServiceClient
}

func (w *memoryPluginWrapper) GetLifecycle() Lifecycle          { return nil }
func (w *memoryPluginWrapper) GetPluginInfo() *proto.PluginInfo { return w.info }
func (w *memoryPluginWrapper) Store(ctx context.Context, req StoreRequest) (*StoreResponse, error) {
	protoReq := &proto.StoreReq{
		Content:   req.Content,
		Namespace: req.Namespace,
	}
	for k, v := range req.Metadata {
		protoReq.Metadata[k] = fmt.Sprintf("%v", v)
	}

	resp, err := w.client.Store(ctx, protoReq)
	if err != nil {
		return nil, err
	}
	return &StoreResponse{ID: resp.Id}, nil
}

func (w *memoryPluginWrapper) Retrieve(ctx context.Context, req RetrieveRequest) (*RetrieveResponse, error) {
	protoReq := &proto.RetrieveReq{
		Query:     req.Query,
		Limit:     int32(req.Limit),
		Namespace: req.Namespace,
	}
	for k, v := range req.Filter {
		protoReq.Filter[k] = fmt.Sprintf("%v", v)
	}

	resp, err := w.client.Retrieve(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	result := &RetrieveResponse{}
	for _, r := range resp.Results {
		metadata := make(map[string]any)
		for k, v := range r.Metadata {
			metadata[k] = v
		}
		result.Results = append(result.Results, MemoryResult{
			ID:       r.Id,
			Content:  r.Content,
			Score:    r.Score,
			Metadata: metadata,
		})
	}
	return result, nil
}

type channelPluginWrapper struct {
	info   *proto.PluginInfo
	client proto.ChannelServiceClient
}

func (w *channelPluginWrapper) GetLifecycle() Lifecycle          { return nil }
func (w *channelPluginWrapper) GetPluginInfo() *proto.PluginInfo { return w.info }
func (w *channelPluginWrapper) Send(ctx context.Context, req SendRequest) (*SendResponse, error) {
	protoReq := &proto.SendReq{
		ChannelId: req.ChannelID,
		Content:   req.Content,
	}
	for k, v := range req.Metadata {
		protoReq.Metadata[k] = fmt.Sprintf("%v", v)
	}

	resp, err := w.client.Send(ctx, protoReq)
	if err != nil {
		return nil, err
	}
	return &SendResponse{MessageID: resp.MessageId}, nil
}

func (w *channelPluginWrapper) Receive(ctx context.Context) (<-chan MessageEvent, error) {
	stream, err := w.client.Receive(ctx, &proto.ReceiveReq{})
	if err != nil {
		return nil, err
	}

	ch := make(chan MessageEvent, 1)
	go func() {
		defer close(ch)
		for {
			protoEvent, err := stream.Recv()
			if err != nil {
				// Log the error but don't propagate it since the channel is closing
				// The caller should check for channel closure to detect stream end
				return
			}
			metadata := make(map[string]any)
			for k, v := range protoEvent.Metadata {
				metadata[k] = v
			}
			ch <- MessageEvent{
				ID:        protoEvent.Id,
				ChannelID: protoEvent.ChannelId,
				AuthorID:  protoEvent.AuthorId,
				Content:   protoEvent.Content,
				Metadata:  metadata,
			}
		}
	}()
	return ch, nil
}

type toolsPluginWrapper struct {
	info   *proto.PluginInfo
	client proto.ToolsServiceClient
}

func (w *toolsPluginWrapper) GetLifecycle() Lifecycle          { return nil }
func (w *toolsPluginWrapper) GetPluginInfo() *proto.PluginInfo { return w.info }
func (w *toolsPluginWrapper) List(ctx context.Context) (*ListToolsResponse, error) {
	resp, err := w.client.List(ctx, &proto.ListToolsReq{})
	if err != nil {
		return nil, err
	}

	result := &ListToolsResponse{}
	for _, t := range resp.Tools {
		params := make(map[string]any)
		for k, v := range t.Parameters {
			params[k] = v
		}
		result.Tools = append(result.Tools, ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return result, nil
}

func (w *toolsPluginWrapper) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	protoReq := &proto.ExecuteReq{
		Tool: req.Tool,
	}
	for k, v := range req.Arguments {
		protoReq.Arguments[k] = fmt.Sprintf("%v", v)
	}

	resp, err := w.client.Execute(ctx, protoReq)
	if err != nil {
		return nil, err
	}

	// Convert proto result map to Go map
	resultMap := make(map[string]any)
	for k, v := range resp.Result {
		resultMap[k] = v
	}

	return &ExecuteResponse{
		Result:  resultMap,
		IsError: resp.IsError,
	}, nil
}

// Server-side gRPC implementations for each plugin type.
// These are registered alongside the DriverService and delegate to the driver's plugins.

// providerGRPCServer implements ProviderServiceServer.
type providerGRPCServer struct {
	proto.UnimplementedProviderServiceServer
	Impl Driver
}

func (s *providerGRPCServer) Generate(ctx context.Context, req *proto.GenerateReq) (*proto.GenerateResp, error) {
	plugin, err := s.Impl.GetProviderPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("provider plugin not available")
	}

	// Convert proto request to plugin request
	generateReq := GenerateRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   int(req.MaxTokens),
	}
	for _, m := range req.Messages {
		generateReq.Messages = append(generateReq.Messages, Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	for _, t := range req.Tools {
		params := make(map[string]interface{})
		for k, v := range t.Parameters {
			params[k] = v
		}
		generateReq.Tools = append(generateReq.Tools, Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}

	resp, err := plugin.Generate(ctx, generateReq)
	if err != nil {
		return nil, err
	}

	// Convert response to proto
	protoResp := &proto.GenerateResp{
		Id:      resp.ID,
		Content: resp.Content,
		Role:    resp.Role,
		Model:   resp.Model,
	}
	if resp.Usage != nil {
		protoResp.Usage = &proto.UsageProto{
			InputTokens:  int32(resp.Usage.InputTokens),
			OutputTokens: int32(resp.Usage.OutputTokens),
		}
	}

	return protoResp, nil
}

// memoryGRPCServer implements MemoryServiceServer.
type memoryGRPCServer struct {
	proto.UnimplementedMemoryServiceServer
	Impl Driver
}

func (s *memoryGRPCServer) Store(ctx context.Context, req *proto.StoreReq) (*proto.StoreResp, error) {
	plugin, err := s.Impl.GetMemoryPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("memory plugin not available")
	}

	metadata := make(map[string]any)
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	resp, err := plugin.Store(ctx, StoreRequest{
		Content:   req.Content,
		Namespace: req.Namespace,
		Metadata:  metadata,
	})
	if err != nil {
		return nil, err
	}

	return &proto.StoreResp{Id: resp.ID}, nil
}

func (s *memoryGRPCServer) Retrieve(ctx context.Context, req *proto.RetrieveReq) (*proto.RetrieveResp, error) {
	plugin, err := s.Impl.GetMemoryPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("memory plugin not available")
	}

	filter := make(map[string]any)
	for k, v := range req.Filter {
		filter[k] = v
	}

	resp, err := plugin.Retrieve(ctx, RetrieveRequest{
		Query:     req.Query,
		Limit:     int(req.Limit),
		Namespace: req.Namespace,
		Filter:    filter,
	})
	if err != nil {
		return nil, err
	}

	protoResp := &proto.RetrieveResp{}
	for _, r := range resp.Results {
		metadata := make(map[string]string)
		for k, v := range r.Metadata {
			metadata[k] = fmt.Sprintf("%v", v)
		}
		protoResp.Results = append(protoResp.Results, &proto.MemoryResultProto{
			Id:       r.ID,
			Content:  r.Content,
			Score:    r.Score,
			Metadata: metadata,
		})
	}

	return protoResp, nil
}

// channelGRPCServer implements ChannelServiceServer.
type channelGRPCServer struct {
	proto.UnimplementedChannelServiceServer
	Impl Driver
}

func (s *channelGRPCServer) Send(ctx context.Context, req *proto.SendReq) (*proto.SendResp, error) {
	plugin, err := s.Impl.GetChannelPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("channel plugin not available")
	}

	metadata := make(map[string]any)
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	resp, err := plugin.Send(ctx, SendRequest{
		ChannelID: req.ChannelId,
		Content:   req.Content,
		Metadata:  metadata,
	})
	if err != nil {
		return nil, err
	}

	return &proto.SendResp{MessageId: resp.MessageID}, nil
}

func (s *channelGRPCServer) Receive(req *proto.ReceiveReq, srv proto.ChannelService_ReceiveServer) error {
	plugin, err := s.Impl.GetProviderPlugin(srv.Context())
	if err != nil {
		return err
	}
	if plugin == nil {
		return fmt.Errorf("channel plugin not available")
	}

	// This is a streaming endpoint - we need to get the channel plugin differently
	// For now, return not implemented
	return fmt.Errorf("streaming receive not yet implemented for gRPC bridge")
}

// toolsGRPCServer implements ToolsServiceServer.
type toolsGRPCServer struct {
	proto.UnimplementedToolsServiceServer
	Impl Driver
}

func (s *toolsGRPCServer) List(ctx context.Context, req *proto.ListToolsReq) (*proto.ListToolsResp, error) {
	plugin, err := s.Impl.GetToolsPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("tools plugin not available")
	}

	resp, err := plugin.List(ctx)
	if err != nil {
		return nil, err
	}

	protoResp := &proto.ListToolsResp{}
	for _, t := range resp.Tools {
		params := make(map[string]string)
		for k, v := range t.Parameters {
			params[k] = fmt.Sprintf("%v", v)
		}
		protoResp.Tools = append(protoResp.Tools, &proto.ToolDefProto{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}

	return protoResp, nil
}

func (s *toolsGRPCServer) Execute(ctx context.Context, req *proto.ExecuteReq) (*proto.ExecuteResp, error) {
	plugin, err := s.Impl.GetToolsPlugin(ctx)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, fmt.Errorf("tools plugin not available")
	}

	args := make(map[string]any)
	for k, v := range req.Arguments {
		args[k] = v
	}

	resp, err := plugin.Execute(ctx, ExecuteRequest{
		Tool:      req.Tool,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	if resultMap, ok := resp.Result.(map[string]any); ok {
		for k, v := range resultMap {
			result[k] = fmt.Sprintf("%v", v)
		}
	}

	return &proto.ExecuteResp{
		Result:  result,
		IsError: resp.IsError,
	}, nil
}
