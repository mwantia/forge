package consul

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/mwantia/forge/pkg/plugins"
)

func (d *ConsulDriver) GetLifecycle() plugins.Lifecycle {
	return d
}

func (d *ConsulDriver) ListTools(_ context.Context, filter plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	tools := make([]plugins.ToolDefinition, 0, len(toolDefinitions))
	for _, def := range toolDefinitions {
		if matchesFilter(def, filter) {
			tools = append(tools, def)
		}
	}
	return &plugins.ListToolsResponse{Tools: tools}, nil
}

func (d *ConsulDriver) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	def, ok := toolDefinitions[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return &def, nil
}

func (d *ConsulDriver) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	var errs []string

	def, ok := toolDefinitions[req.Tool]
	if !ok {
		return &plugins.ValidateResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("unknown tool %q", req.Tool)},
		}, nil
	}

	params, _ := def.Parameters["properties"].(map[string]any)
	required, _ := def.Parameters["required"].([]string)
	for _, r := range required {
		v, exists := req.Arguments[r]
		if !exists || v == nil || v == "" {
			errs = append(errs, fmt.Sprintf("%q is required", r))
			continue
		}
		if paramDef, ok := params[r].(map[string]any); ok {
			if enum, ok := paramDef["enum"].([]string); ok {
				val, _ := v.(string)
				if !containsStr(enum, val) {
					errs = append(errs, fmt.Sprintf("%q must be one of: %s", r, strings.Join(enum, ", ")))
				}
			}
		}
	}

	return &plugins.ValidateResponse{Valid: len(errs) == 0, Errors: errs}, nil
}

func (d *ConsulDriver) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}
	if d.client == nil {
		return &plugins.ExecuteResponse{Result: "consul client not initialized", IsError: true}, nil
	}

	switch req.Tool {
	case "catalog_datacenters":
		return d.execCatalogDatacenters(ctx, req.Arguments)
	case "catalog_nodes":
		return d.execCatalogNodes(ctx, req.Arguments)
	case "catalog_services":
		return d.execCatalogServices(ctx, req.Arguments)
	case "catalog_service":
		return d.execCatalogService(ctx, req.Arguments)
	case "catalog_node":
		return d.execCatalogNode(ctx, req.Arguments)
	case "health_service":
		return d.execHealthService(ctx, req.Arguments)
	case "health_node":
		return d.execHealthNode(ctx, req.Arguments)
	case "health_checks":
		return d.execHealthChecks(ctx, req.Arguments)
	case "kv_get":
		return d.execKVGet(ctx, req.Arguments)
	case "kv_list":
		return d.execKVList(ctx, req.Arguments)
	case "kv_put":
		return d.execKVPut(ctx, req.Arguments)
	case "kv_delete":
		return d.execKVDelete(ctx, req.Arguments)
	case "agent_members":
		return d.execAgentMembers(ctx, req.Arguments)
	case "agent_services":
		return d.execAgentServices(ctx, req.Arguments)
	default:
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("unknown tool: %s", req.Tool),
			IsError: true,
		}, nil
	}
}

func (d *ConsulDriver) execCatalogDatacenters(_ context.Context, _ map[string]any) (*plugins.ExecuteResponse, error) {
	dcs, err := d.client.Catalog().Datacenters()
	if err != nil {
		return errorResponse("catalog_datacenters failed: %v", err), nil
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"datacenters": dcs}}, nil
}

func (d *ConsulDriver) execCatalogNodes(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	q := queryOptions(args)
	if f, ok := args["filter"].(string); ok && f != "" {
		q.Filter = f
	}

	nodes, _, err := d.client.Catalog().Nodes(q)
	if err != nil {
		return errorResponse("catalog_nodes failed: %v", err), nil
	}

	type nodeEntry struct {
		ID              string            `json:"id"`
		Node            string            `json:"node"`
		Address         string            `json:"address"`
		Datacenter      string            `json:"datacenter"`
		TaggedAddresses map[string]string `json:"tagged_addresses,omitempty"`
		Meta            map[string]string `json:"meta,omitempty"`
	}
	entries := make([]nodeEntry, 0, len(nodes))
	for _, n := range nodes {
		entries = append(entries, nodeEntry{
			ID:              n.ID,
			Node:            n.Node,
			Address:         n.Address,
			Datacenter:      n.Datacenter,
			TaggedAddresses: n.TaggedAddresses,
			Meta:            n.Meta,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"nodes": entries, "count": len(entries)}}, nil
}

func (d *ConsulDriver) execCatalogServices(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	q := queryOptions(args)

	services, _, err := d.client.Catalog().Services(q)
	if err != nil {
		return errorResponse("catalog_services failed: %v", err), nil
	}

	type serviceEntry struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	entries := make([]serviceEntry, 0, len(services))
	for name, tags := range services {
		entries = append(entries, serviceEntry{Name: name, Tags: tags})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"services": entries, "count": len(entries)}}, nil
}

func (d *ConsulDriver) execCatalogService(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	service, _ := args["service"].(string)
	if service == "" {
		return &plugins.ExecuteResponse{Result: `"service" is required`, IsError: true}, nil
	}

	tag, _ := args["tag"].(string)
	q := queryOptions(args)

	entries, _, err := d.client.Catalog().Service(service, tag, q)
	if err != nil {
		return errorResponse("catalog_service failed: %v", err), nil
	}

	type serviceNode struct {
		Node           string            `json:"node"`
		NodeAddress    string            `json:"node_address"`
		Datacenter     string            `json:"datacenter"`
		ServiceID      string            `json:"service_id"`
		ServiceName    string            `json:"service_name"`
		ServiceAddress string            `json:"service_address"`
		ServicePort    int               `json:"service_port"`
		ServiceTags    []string          `json:"service_tags"`
		ServiceMeta    map[string]string `json:"service_meta,omitempty"`
	}
	result := make([]serviceNode, 0, len(entries))
	for _, e := range entries {
		result = append(result, serviceNode{
			Node:           e.Node,
			NodeAddress:    e.Address,
			Datacenter:     e.Datacenter,
			ServiceID:      e.ServiceID,
			ServiceName:    e.ServiceName,
			ServiceAddress: e.ServiceAddress,
			ServicePort:    e.ServicePort,
			ServiceTags:    e.ServiceTags,
			ServiceMeta:    e.ServiceMeta,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"service": service, "nodes": result, "count": len(result)}}, nil
}

func (d *ConsulDriver) execCatalogNode(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	node, _ := args["node"].(string)
	if node == "" {
		return &plugins.ExecuteResponse{Result: `"node" is required`, IsError: true}, nil
	}

	q := queryOptions(args)
	info, _, err := d.client.Catalog().Node(node, q)
	if err != nil {
		return errorResponse("catalog_node failed: %v", err), nil
	}
	if info == nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("node %q not found", node), IsError: true}, nil
	}

	type serviceEntry struct {
		ID      string            `json:"id"`
		Name    string            `json:"name"`
		Address string            `json:"address"`
		Port    int               `json:"port"`
		Tags    []string          `json:"tags"`
		Meta    map[string]string `json:"meta,omitempty"`
	}
	services := make([]serviceEntry, 0, len(info.Services))
	for _, svc := range info.Services {
		services = append(services, serviceEntry{
			ID:      svc.ID,
			Name:    svc.Service,
			Address: svc.Address,
			Port:    svc.Port,
			Tags:    svc.Tags,
			Meta:    svc.Meta,
		})
	}

	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"node":     info.Node.Node,
			"address":  info.Node.Address,
			"services": services,
		},
	}, nil
}

func (d *ConsulDriver) execHealthService(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	service, _ := args["service"].(string)
	if service == "" {
		return &plugins.ExecuteResponse{Result: `"service" is required`, IsError: true}, nil
	}

	tag, _ := args["tag"].(string)
	passingOnly, _ := args["passing_only"].(bool)
	q := queryOptions(args)

	entries, _, err := d.client.Health().Service(service, tag, passingOnly, q)
	if err != nil {
		return errorResponse("health_service failed: %v", err), nil
	}

	type checkSummary struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Output string `json:"output,omitempty"`
	}
	type serviceHealth struct {
		Node        string         `json:"node"`
		NodeAddress string         `json:"node_address"`
		ServiceID   string         `json:"service_id"`
		ServiceName string         `json:"service_name"`
		ServicePort int            `json:"service_port"`
		Checks      []checkSummary `json:"checks"`
	}
	result := make([]serviceHealth, 0, len(entries))
	for _, e := range entries {
		checks := make([]checkSummary, 0, len(e.Checks))
		for _, c := range e.Checks {
			checks = append(checks, checkSummary{
				Name:   c.Name,
				Status: c.Status,
				Output: c.Output,
			})
		}
		result = append(result, serviceHealth{
			Node:        e.Node.Node,
			NodeAddress: e.Node.Address,
			ServiceID:   e.Service.ID,
			ServiceName: e.Service.Service,
			ServicePort: e.Service.Port,
			Checks:      checks,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"service": service, "instances": result, "count": len(result)}}, nil
}

func (d *ConsulDriver) execHealthNode(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	node, _ := args["node"].(string)
	if node == "" {
		return &plugins.ExecuteResponse{Result: `"node" is required`, IsError: true}, nil
	}

	q := queryOptions(args)
	checks, _, err := d.client.Health().Node(node, q)
	if err != nil {
		return errorResponse("health_node failed: %v", err), nil
	}

	type checkEntry struct {
		CheckID     string `json:"check_id"`
		Name        string `json:"name"`
		ServiceID   string `json:"service_id,omitempty"`
		ServiceName string `json:"service_name,omitempty"`
		Status      string `json:"status"`
		Output      string `json:"output,omitempty"`
	}
	entries := make([]checkEntry, 0, len(checks))
	for _, c := range checks {
		entries = append(entries, checkEntry{
			CheckID:     c.CheckID,
			Name:        c.Name,
			ServiceID:   c.ServiceID,
			ServiceName: c.ServiceName,
			Status:      c.Status,
			Output:      c.Output,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"node": node, "checks": entries, "count": len(entries)}}, nil
}

func (d *ConsulDriver) execHealthChecks(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	state, _ := args["state"].(string)
	if state == "" {
		state = "any"
	}

	q := queryOptions(args)
	checks, _, err := d.client.Health().State(state, q)
	if err != nil {
		return errorResponse("health_checks failed: %v", err), nil
	}

	type checkEntry struct {
		Node        string `json:"node"`
		CheckID     string `json:"check_id"`
		Name        string `json:"name"`
		ServiceID   string `json:"service_id,omitempty"`
		ServiceName string `json:"service_name,omitempty"`
		Status      string `json:"status"`
		Output      string `json:"output,omitempty"`
	}
	entries := make([]checkEntry, 0, len(checks))
	for _, c := range checks {
		entries = append(entries, checkEntry{
			Node:        c.Node,
			CheckID:     c.CheckID,
			Name:        c.Name,
			ServiceID:   c.ServiceID,
			ServiceName: c.ServiceName,
			Status:      c.Status,
			Output:      c.Output,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"state": state, "checks": entries, "count": len(entries)}}, nil
}

func (d *ConsulDriver) execKVGet(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	key, _ := args["key"].(string)
	if key == "" {
		return &plugins.ExecuteResponse{Result: `"key" is required`, IsError: true}, nil
	}

	q := queryOptions(args)
	pair, _, err := d.client.KV().Get(key, q)
	if err != nil {
		return errorResponse("kv_get failed: %v", err), nil
	}
	if pair == nil {
		return &plugins.ExecuteResponse{Result: map[string]any{"key": key, "found": false}}, nil
	}

	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"key":          pair.Key,
			"value":        string(pair.Value),
			"flags":        pair.Flags,
			"session":      pair.Session,
			"modify_index": pair.ModifyIndex,
			"found":        true,
		},
	}, nil
}

func (d *ConsulDriver) execKVList(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	prefix, _ := args["prefix"].(string)
	q := queryOptions(args)

	keys, _, err := d.client.KV().Keys(prefix, "", q)
	if err != nil {
		return errorResponse("kv_list failed: %v", err), nil
	}

	return &plugins.ExecuteResponse{Result: map[string]any{"prefix": prefix, "keys": keys, "count": len(keys)}}, nil
}

func (d *ConsulDriver) execKVPut(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	key, _ := args["key"].(string)
	value, _ := args["value"].(string)
	if key == "" {
		return &plugins.ExecuteResponse{Result: `"key" is required`, IsError: true}, nil
	}

	q := writeOptions(args)
	p := &api.KVPair{Key: key, Value: []byte(value)}
	_, err := d.client.KV().Put(p, q)
	if err != nil {
		return errorResponse("kv_put failed: %v", err), nil
	}

	d.log.Info("KV put", "key", key)
	return &plugins.ExecuteResponse{Result: map[string]any{"key": key, "ok": true}}, nil
}

func (d *ConsulDriver) execKVDelete(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	key, _ := args["key"].(string)
	if key == "" {
		return &plugins.ExecuteResponse{Result: `"key" is required`, IsError: true}, nil
	}

	recurse, _ := args["recurse"].(bool)
	q := writeOptions(args)

	var err error
	if recurse {
		_, err = d.client.KV().DeleteTree(key, q)
	} else {
		_, err = d.client.KV().Delete(key, q)
	}
	if err != nil {
		return errorResponse("kv_delete failed: %v", err), nil
	}

	d.log.Info("KV delete", "key", key, "recurse", recurse)
	return &plugins.ExecuteResponse{Result: map[string]any{"key": key, "recurse": recurse, "ok": true}}, nil
}

func (d *ConsulDriver) execAgentMembers(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	wan, _ := args["wan"].(bool)
	members, err := d.client.Agent().Members(wan)
	if err != nil {
		return errorResponse("agent_members failed: %v", err), nil
	}

	type memberEntry struct {
		Name   string            `json:"name"`
		Addr   string            `json:"address"`
		Port   uint16            `json:"port"`
		Status int               `json:"status"`
		Tags   map[string]string `json:"tags,omitempty"`
	}
	entries := make([]memberEntry, 0, len(members))
	for _, m := range members {
		entries = append(entries, memberEntry{
			Name:   m.Name,
			Addr:   m.Addr,
			Port:   m.Port,
			Status: m.Status,
			Tags:   m.Tags,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"members": entries, "count": len(entries), "wan": wan}}, nil
}

func (d *ConsulDriver) execAgentServices(_ context.Context, _ map[string]any) (*plugins.ExecuteResponse, error) {
	services, err := d.client.Agent().Services()
	if err != nil {
		return errorResponse("agent_services failed: %v", err), nil
	}

	type serviceEntry struct {
		ID      string            `json:"id"`
		Name    string            `json:"service"`
		Address string            `json:"address"`
		Port    int               `json:"port"`
		Tags    []string          `json:"tags"`
		Meta    map[string]string `json:"meta,omitempty"`
	}
	entries := make([]serviceEntry, 0, len(services))
	for _, svc := range services {
		entries = append(entries, serviceEntry{
			ID:      svc.ID,
			Name:    svc.Service,
			Address: svc.Address,
			Port:    svc.Port,
			Tags:    svc.Tags,
			Meta:    svc.Meta,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"services": entries, "count": len(entries)}}, nil
}

// queryOptions builds a Consul QueryOptions from common tool arguments.
func queryOptions(args map[string]any) *api.QueryOptions {
	q := &api.QueryOptions{}
	if dc, ok := args["datacenter"].(string); ok && dc != "" {
		q.Datacenter = dc
	}
	return q
}

// writeOptions builds a Consul WriteOptions from common tool arguments.
func writeOptions(args map[string]any) *api.WriteOptions {
	w := &api.WriteOptions{}
	if dc, ok := args["datacenter"].(string); ok && dc != "" {
		w.Datacenter = dc
	}
	return w
}

func errorResponse(format string, args ...any) *plugins.ExecuteResponse {
	return &plugins.ExecuteResponse{
		Result:  fmt.Sprintf(format, args...),
		IsError: true,
	}
}

func matchesFilter(def plugins.ToolDefinition, f plugins.ListToolsFilter) bool {
	if def.Deprecated && !f.Deprecated {
		return false
	}
	if f.Prefix != "" && !strings.HasPrefix(def.Name, f.Prefix) {
		return false
	}
	if len(f.Tags) > 0 {
		for _, want := range f.Tags {
			for _, have := range def.Tags {
				if have == want {
					goto tagMatched
				}
			}
		}
		return false
	tagMatched:
	}
	return true
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
