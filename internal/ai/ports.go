package ai

import "github.com/ilaziness/vela/internal/ai/tool"

// RunDeps wires per-run tool ports and engine-only hooks.
type RunDeps struct {
	Tools        tool.Deps
	OnCheckpoint func(turn []ChatMessage)
}

// Compatibility aliases for older call sites / tests.
type (
	SSHExecutor  = tool.SSHExecutor
	ApprovalGate = tool.ApprovalGate
	ToolDeps     = tool.Deps
)

const (
	ToolRunSSHCommand = tool.RunSSHCommand
	ToolUpsertPlan    = tool.UpsertPlan
)
