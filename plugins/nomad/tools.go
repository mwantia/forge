package nomad

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/mwantia/forge/pkg/plugins"
)

func (d *NomadDriver) GetLifecycle() plugins.Lifecycle {
	return d
}

func (d *NomadDriver) ListTools(_ context.Context, filter plugins.ListToolsFilter) (*plugins.ListToolsResponse, error) {
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

func (d *NomadDriver) GetTool(_ context.Context, name string) (*plugins.ToolDefinition, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	def, ok := toolDefinitions[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return &def, nil
}

func (d *NomadDriver) Validate(_ context.Context, req plugins.ExecuteRequest) (*plugins.ValidateResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}

	def, ok := toolDefinitions[req.Tool]
	if !ok {
		return &plugins.ValidateResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("unknown tool %q", req.Tool)},
		}, nil
	}

	var errs []string
	required, _ := def.Parameters["required"].([]string)
	for _, r := range required {
		v, exists := req.Arguments[r]
		if !exists || v == nil || v == "" {
			errs = append(errs, fmt.Sprintf("%q is required", r))
		}
	}

	return &plugins.ValidateResponse{Valid: len(errs) == 0, Errors: errs}, nil
}

func (d *NomadDriver) Execute(ctx context.Context, req plugins.ExecuteRequest) (*plugins.ExecuteResponse, error) {
	if d.config == nil {
		return nil, fmt.Errorf("plugin not configured")
	}
	if d.client == nil {
		return &plugins.ExecuteResponse{Result: "nomad client not initialized", IsError: true}, nil
	}

	switch req.Tool {
	case "jobs_list":
		return d.execJobsList(ctx, req.Arguments)
	case "job_get":
		return d.execJobGet(ctx, req.Arguments)
	case "job_summary":
		return d.execJobSummary(ctx, req.Arguments)
	case "job_submit":
		return d.execJobSubmit(ctx, req.Arguments)
	case "job_stop":
		return d.execJobStop(ctx, req.Arguments)
	case "allocations_list":
		return d.execAllocationsList(ctx, req.Arguments)
	case "allocation_get":
		return d.execAllocationGet(ctx, req.Arguments)
	case "nodes_list":
		return d.execNodesList(ctx, req.Arguments)
	case "node_get":
		return d.execNodeGet(ctx, req.Arguments)
	case "evaluations_list":
		return d.execEvaluationsList(ctx, req.Arguments)
	case "namespaces_list":
		return d.execNamespacesList(ctx, req.Arguments)
	case "agent_members":
		return d.execAgentMembers(ctx, req.Arguments)
	case "agent_self":
		return d.execAgentSelf(ctx, req.Arguments)
	default:
		return &plugins.ExecuteResponse{
			Result:  fmt.Sprintf("unknown tool: %s", req.Tool),
			IsError: true,
		}, nil
	}
}

func (d *NomadDriver) execJobsList(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	q := queryOptions(args)
	if prefix, ok := args["prefix"].(string); ok && prefix != "" {
		q.Prefix = prefix
	}

	jobs, _, err := d.client.Jobs().List(q)
	if err != nil {
		return errorResponse("jobs_list failed: %v", err), nil
	}

	type jobEntry struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		Status      string `json:"status"`
		Namespace   string `json:"namespace"`
		Datacenters []string `json:"datacenters"`
		Priority    int    `json:"priority"`
	}
	entries := make([]jobEntry, 0, len(jobs))
	for _, j := range jobs {
		entries = append(entries, jobEntry{
			ID:          j.ID,
			Name:        j.Name,
			Type:        j.Type,
			Status:      j.Status,
			Namespace:   j.Namespace,
			Datacenters: j.Datacenters,
			Priority:    j.Priority,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"jobs": entries, "count": len(entries)}}, nil
}

func (d *NomadDriver) execJobGet(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	jobID, _ := args["job_id"].(string)
	if jobID == "" {
		return &plugins.ExecuteResponse{Result: `"job_id" is required`, IsError: true}, nil
	}

	q := queryOptions(args)
	job, _, err := d.client.Jobs().Info(jobID, q)
	if err != nil {
		return errorResponse("job_get failed: %v", err), nil
	}

	return &plugins.ExecuteResponse{Result: job}, nil
}

func (d *NomadDriver) execJobSummary(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	jobID, _ := args["job_id"].(string)
	if jobID == "" {
		return &plugins.ExecuteResponse{Result: `"job_id" is required`, IsError: true}, nil
	}

	q := queryOptions(args)
	summary, _, err := d.client.Jobs().Summary(jobID, q)
	if err != nil {
		return errorResponse("job_summary failed: %v", err), nil
	}

	type groupSummary struct {
		Queued   int `json:"queued"`
		Complete int `json:"complete"`
		Failed   int `json:"failed"`
		Running  int `json:"running"`
		Starting int `json:"starting"`
		Lost     int `json:"lost"`
		Unknown  int `json:"unknown"`
	}
	groups := make(map[string]groupSummary, len(summary.Summary))
	for name, s := range summary.Summary {
		groups[name] = groupSummary{
			Queued:   s.Queued,
			Complete: s.Complete,
			Failed:   s.Failed,
			Running:  s.Running,
			Starting: s.Starting,
			Lost:     s.Lost,
			Unknown:  s.Unknown,
		}
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"job_id": summary.JobID, "groups": groups}}, nil
}

func (d *NomadDriver) execJobSubmit(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	spec, _ := args["job_spec"].(string)
	if spec == "" {
		return &plugins.ExecuteResponse{Result: `"job_spec" is required`, IsError: true}, nil
	}

	var job api.Job
	if err := json.Unmarshal([]byte(spec), &job); err != nil {
		return &plugins.ExecuteResponse{Result: fmt.Sprintf("invalid job_spec JSON: %v", err), IsError: true}, nil
	}

	resp, _, err := d.client.Jobs().Register(&job, nil)
	if err != nil {
		return errorResponse("job_submit failed: %v", err), nil
	}

	d.log.Info("Job submitted", "job_id", job.ID, "eval_id", resp.EvalID)
	return &plugins.ExecuteResponse{Result: map[string]any{"eval_id": resp.EvalID, "warnings": resp.Warnings}}, nil
}

func (d *NomadDriver) execJobStop(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	jobID, _ := args["job_id"].(string)
	if jobID == "" {
		return &plugins.ExecuteResponse{Result: `"job_id" is required`, IsError: true}, nil
	}

	purge, _ := args["purge"].(bool)
	q := writeOptions(args)

	resp, _, err := d.client.Jobs().Deregister(jobID, purge, q)
	if err != nil {
		return errorResponse("job_stop failed: %v", err), nil
	}

	d.log.Info("Job stopped", "job_id", jobID, "purge", purge)
	return &plugins.ExecuteResponse{Result: map[string]any{"job_id": jobID, "eval_id": resp, "purge": purge}}, nil
}

func (d *NomadDriver) execAllocationsList(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	q := queryOptions(args)

	var stubs []*api.AllocationListStub
	var err error

	if jobID, ok := args["job_id"].(string); ok && jobID != "" {
		stubs, _, err = d.client.Jobs().Allocations(jobID, false, q)
	} else {
		stubs, _, err = d.client.Allocations().List(q)
	}
	if err != nil {
		return errorResponse("allocations_list failed: %v", err), nil
	}

	type allocEntry struct {
		ID            string `json:"id"`
		JobID         string `json:"job_id"`
		TaskGroup     string `json:"task_group"`
		NodeID        string `json:"node_id"`
		NodeName      string `json:"node_name"`
		ClientStatus  string `json:"client_status"`
		DesiredStatus string `json:"desired_status"`
		Namespace     string `json:"namespace"`
	}
	entries := make([]allocEntry, 0, len(stubs))
	for _, a := range stubs {
		entries = append(entries, allocEntry{
			ID:            a.ID,
			JobID:         a.JobID,
			TaskGroup:     a.TaskGroup,
			NodeID:        a.NodeID,
			NodeName:      a.NodeName,
			ClientStatus:  a.ClientStatus,
			DesiredStatus: a.DesiredStatus,
			Namespace:     a.Namespace,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"allocations": entries, "count": len(entries)}}, nil
}

func (d *NomadDriver) execAllocationGet(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	allocID, _ := args["alloc_id"].(string)
	if allocID == "" {
		return &plugins.ExecuteResponse{Result: `"alloc_id" is required`, IsError: true}, nil
	}

	alloc, _, err := d.client.Allocations().Info(allocID, nil)
	if err != nil {
		return errorResponse("allocation_get failed: %v", err), nil
	}

	type taskState struct {
		State   string `json:"state"`
		Failed  bool   `json:"failed"`
		Restarts uint64 `json:"restarts"`
	}
	tasks := make(map[string]taskState, len(alloc.TaskStates))
	for name, ts := range alloc.TaskStates {
		tasks[name] = taskState{
			State:    ts.State,
			Failed:   ts.Failed,
			Restarts: ts.Restarts,
		}
	}

	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"id":             alloc.ID,
			"job_id":         alloc.JobID,
			"task_group":     alloc.TaskGroup,
			"node_id":        alloc.NodeID,
			"client_status":  alloc.ClientStatus,
			"desired_status": alloc.DesiredStatus,
			"namespace":      alloc.Namespace,
			"task_states":    tasks,
		},
	}, nil
}

func (d *NomadDriver) execNodesList(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	q := &api.QueryOptions{}
	if prefix, ok := args["prefix"].(string); ok && prefix != "" {
		q.Prefix = prefix
	}

	nodes, _, err := d.client.Nodes().List(q)
	if err != nil {
		return errorResponse("nodes_list failed: %v", err), nil
	}

	type nodeEntry struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Datacenter string `json:"datacenter"`
		Status     string `json:"status"`
		Drain      bool   `json:"drain"`
		Version    string `json:"version"`
	}
	entries := make([]nodeEntry, 0, len(nodes))
	for _, n := range nodes {
		entries = append(entries, nodeEntry{
			ID:         n.ID,
			Name:       n.Name,
			Datacenter: n.Datacenter,
			Status:     n.Status,
			Drain:      n.Drain,
			Version:    n.Version,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"nodes": entries, "count": len(entries)}}, nil
}

func (d *NomadDriver) execNodeGet(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	nodeID, _ := args["node_id"].(string)
	if nodeID == "" {
		return &plugins.ExecuteResponse{Result: `"node_id" is required`, IsError: true}, nil
	}

	node, _, err := d.client.Nodes().Info(nodeID, nil)
	if err != nil {
		return errorResponse("node_get failed: %v", err), nil
	}

	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"id":         node.ID,
			"name":       node.Name,
			"datacenter": node.Datacenter,
			"status":     node.Status,
			"drain":      node.Drain,
			"version":    node.Attributes["nomad.version"],
			"attributes": node.Attributes,
			"meta":       node.Meta,
		},
	}, nil
}

func (d *NomadDriver) execEvaluationsList(_ context.Context, args map[string]any) (*plugins.ExecuteResponse, error) {
	q := queryOptions(args)

	var evals []*api.Evaluation
	var err error

	if jobID, ok := args["job_id"].(string); ok && jobID != "" {
		evals, _, err = d.client.Jobs().Evaluations(jobID, q)
	} else {
		evals, _, err = d.client.Evaluations().List(q)
	}
	if err != nil {
		return errorResponse("evaluations_list failed: %v", err), nil
	}

	type evalEntry struct {
		ID          string `json:"id"`
		JobID       string `json:"job_id"`
		Status      string `json:"status"`
		Type        string `json:"type"`
		TriggeredBy string `json:"triggered_by"`
		Namespace   string `json:"namespace"`
	}
	entries := make([]evalEntry, 0, len(evals))
	for _, e := range evals {
		entries = append(entries, evalEntry{
			ID:          e.ID,
			JobID:       e.JobID,
			Status:      e.Status,
			Type:        e.Type,
			TriggeredBy: e.TriggeredBy,
			Namespace:   e.Namespace,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"evaluations": entries, "count": len(entries)}}, nil
}

func (d *NomadDriver) execNamespacesList(_ context.Context, _ map[string]any) (*plugins.ExecuteResponse, error) {
	namespaces, _, err := d.client.Namespaces().List(nil)
	if err != nil {
		return errorResponse("namespaces_list failed: %v", err), nil
	}

	type nsEntry struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	entries := make([]nsEntry, 0, len(namespaces))
	for _, ns := range namespaces {
		entries = append(entries, nsEntry{
			Name:        ns.Name,
			Description: ns.Description,
		})
	}
	return &plugins.ExecuteResponse{Result: map[string]any{"namespaces": entries, "count": len(entries)}}, nil
}

func (d *NomadDriver) execAgentMembers(_ context.Context, _ map[string]any) (*plugins.ExecuteResponse, error) {
	members, err := d.client.Agent().Members()
	if err != nil {
		return errorResponse("agent_members failed: %v", err), nil
	}

	type memberEntry struct {
		Name   string            `json:"name"`
		Addr   string            `json:"address"`
		Port   uint16            `json:"port"`
		Status string            `json:"status"`
		Tags   map[string]string `json:"tags,omitempty"`
	}
	entries := make([]memberEntry, 0, len(members.Members))
	for _, m := range members.Members {
		entries = append(entries, memberEntry{
			Name:   m.Name,
			Addr:   m.Addr,
			Port:   m.Port,
			Status: m.Status,
			Tags:   m.Tags,
		})
	}
	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"server_name":   members.ServerName,
			"server_region": members.ServerRegion,
			"members":       entries,
			"count":         len(entries),
		},
	}, nil
}

func (d *NomadDriver) execAgentSelf(_ context.Context, _ map[string]any) (*plugins.ExecuteResponse, error) {
	self, err := d.client.Agent().Self()
	if err != nil {
		return errorResponse("agent_self failed: %v", err), nil
	}

	return &plugins.ExecuteResponse{
		Result: map[string]any{
			"member": map[string]any{
				"name":   self.Member.Name,
				"addr":   self.Member.Addr,
				"port":   self.Member.Port,
				"status": self.Member.Status,
				"tags":   self.Member.Tags,
			},
			"stats": self.Stats,
		},
	}, nil
}

// queryOptions builds Nomad QueryOptions from common tool arguments.
func queryOptions(args map[string]any) *api.QueryOptions {
	q := &api.QueryOptions{}
	if ns, ok := args["namespace"].(string); ok && ns != "" {
		q.Namespace = ns
	}
	return q
}

// writeOptions builds Nomad WriteOptions from common tool arguments.
func writeOptions(args map[string]any) *api.WriteOptions {
	w := &api.WriteOptions{}
	if ns, ok := args["namespace"].(string); ok && ns != "" {
		w.Namespace = ns
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
