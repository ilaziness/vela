package tool

import (
	"context"
	"strings"

	"github.com/ilaziness/vela/internal/ai/event"
)

type depsCtxKey struct{}

// SSHExecutor runs remote commands and optional terminal annotations.
type SSHExecutor interface {
	Exec(ctx context.Context, linkID, command string) (output string, err error)
	Annotate(ctx context.Context, linkID, notice string) error
}

// ApprovalGate parks a tool until the UI responds.
type ApprovalGate interface {
	Open(approvalID string)
	Wait(ctx context.Context, approvalID string) (approved bool, reason string, err error)
}

// WebSearchRuntime is decrypted search config for one Run.
type WebSearchRuntime struct {
	Provider SearchProvider
	APIKey   string
}

// Deps is injected per Run via context (not shared engine state).
type Deps struct {
	LinkID   string
	Echo     bool
	SSH      SSHExecutor
	Approval ApprovalGate
	Emit     func(event.StreamEvent)

	MarkSSHApproval  func(approvalID string)
	ArmSSHExecCancel func()

	AllowedDirs []string
	WebSearch   WebSearchRuntime
}

// WithDeps stores Deps on ctx for tool handlers and MCP wrap middleware.
func WithDeps(ctx context.Context, d Deps) context.Context {
	return context.WithValue(ctx, depsCtxKey{}, d)
}

// FromCtx returns Deps from ctx, or zero value.
func FromCtx(ctx context.Context) Deps {
	if ctx == nil {
		return Deps{}
	}
	d, _ := ctx.Value(depsCtxKey{}).(Deps)
	return d
}

func (d Deps) EnableSSH() bool {
	return d.LinkID != "" && d.SSH != nil && d.Approval != nil
}

func (d Deps) EnableFS() bool {
	return len(d.AllowedDirs) > 0 && d.Approval != nil
}

func (d Deps) EnableWebSearch() bool {
	p := d.WebSearch.Provider
	return ValidSearchProvider(p) && strings.TrimSpace(d.WebSearch.APIKey) != ""
}

func emit(deps Deps, ev event.StreamEvent) {
	if deps.Emit != nil {
		deps.Emit(ev)
	}
}
