package ai

import "context"

type toolDepsCtxKey struct{}

// SSHExecutor runs remote commands and optional terminal annotations.
// Implemented by services layer; never imports Wails.
type SSHExecutor interface {
	Exec(ctx context.Context, linkID, command string) (output string, err error)
	// Annotate is display-only (no remote execution). May be a no-op when echo is off.
	Annotate(ctx context.Context, linkID, notice string) error
}

// ApprovalGate parks a tool until the UI responds.
type ApprovalGate interface {
	// Open registers approvalID before the UI event is emitted so a fast
	// RespondToolApproval cannot miss the channel.
	Open(approvalID string)
	Wait(ctx context.Context, approvalID string) (approved bool, reason string, err error)
}

// ToolDeps is injected per Run via context (not shared engine state).
type ToolDeps struct {
	LinkID   string
	Echo     bool
	SSH      SSHExecutor
	Approval ApprovalGate
	Emit     func(StreamEvent)
	// MarkSSHApproval tags an approvalID so RespondToolApproval can arm CancelSSHExec
	// as soon as the user approves (before Wait returns / Exec registers). MCP must not call.
	MarkSSHApproval func(approvalID string)
	// ArmSSHExecCancel arms CancelSSHExec for the approve→Exec race (SSH only).
	// Optional; MCP tools must not call it. Kept as a backup after Wait returns.
	ArmSSHExecCancel func()
	// OnCheckpoint receives the Genkit turn slice (History after promptLen) after each WrapGenerate.
	// Optional; used by services for durable mid-run persistence. Tools must not call it.
	OnCheckpoint func(turn []ChatMessage)
}

func withToolDeps(ctx context.Context, d ToolDeps) context.Context {
	return context.WithValue(ctx, toolDepsCtxKey{}, d)
}

func depsFromCtx(ctx context.Context) ToolDeps {
	if ctx == nil {
		return ToolDeps{}
	}
	d, _ := ctx.Value(toolDepsCtxKey{}).(ToolDeps)
	return d
}

func (d ToolDeps) EnableSSHTool() bool {
	return d.LinkID != "" && d.SSH != nil && d.Approval != nil
}
