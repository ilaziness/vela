package ai

import (
	"context"
	"encoding/json"

	"github.com/ilaziness/vela/internal/ai/tool"
)

// PendingApproval is one ToolApproval hold waiting for the UI.
type PendingApproval struct {
	ApprovalID string
	ToolCallID string
	ToolName   string
	Input      json.RawMessage
}

// ApprovalDecision is the UI answer for one PendingApproval.
type ApprovalDecision struct {
	ApprovalID string
	Approved   bool
	Reason     string
}

// RunDeps wires per-run tool ports and engine-only hooks.
type RunDeps struct {
	Tools tool.Deps
	// WaitApprovals blocks until every pending id has a decision, or ctx is done.
	// Implementations must register ids so RespondToolApproval can resolve them.
	WaitApprovals func(ctx context.Context, pending []PendingApproval) ([]ApprovalDecision, error)
	OnCheckpoint  func(turn []ChatMessage)
}

// Compatibility aliases for older call sites / tests.
type (
	SSHExecutor = tool.SSHExecutor
	ToolDeps    = tool.Deps
)

const (
	ToolRunSSHCommand = tool.RunSSHCommand
	ToolUpsertPlan    = tool.UpsertPlan
)
