package approvals

import (
	"context"
	"time"

	sdkplugins "github.com/mwantia/forge-sdk/pkg/plugin"
)

type ApprovalStatus string

const (
	StatusPending   ApprovalStatus = "pending"
	StatusAllowed   ApprovalStatus = "allowed"
	StatusDenied    ApprovalStatus = "denied"
	StatusTimeout   ApprovalStatus = "timeout"
	StatusCancelled ApprovalStatus = "cancelled"
)

type ApprovalMeta struct {
	Plugin   string
	ToolName string
	ToolArgs map[string]any
}

type ApprovalRecord struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	Status    ApprovalStatus `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	ExpiresAt *time.Time     `json:"expires_at,omitempty"`
	Plugin    string         `json:"plugin,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	ToolArgs  map[string]any `json:"tool_args,omitempty"`
	Reason    string         `json:"reason,omitempty"`
}

type ApprovalFilter struct {
	Status []ApprovalStatus
	Plugin string
}

// ApprovalRegistar is the interface injected by other services.
type ApprovalRegistar interface {
	Create(ctx context.Context, req sdkplugins.ApprovalRequest, meta ApprovalMeta) (*ApprovalRecord, error)
	Respond(id string, decision sdkplugins.ApprovalDecision) error
	Get(id string) (*ApprovalRecord, error)
	List(filter ApprovalFilter) ([]*ApprovalRecord, error)
	Cancel(id string) error
	Subscribe(ctx context.Context) <-chan *ApprovalRecord
	CheckDeny(tool string) bool
	CheckAllow(tool string) bool
}
