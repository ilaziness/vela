package tool

import (
	"context"
	"strings"
	"sync"

	"github.com/ilaziness/vela/internal/ai/event"
)

type depsCtxKey struct{}

// SSHExecutor runs remote commands and optional terminal annotations.
type SSHExecutor interface {
	Exec(ctx context.Context, linkID, command string) (output string, err error)
	Annotate(ctx context.Context, linkID, notice string) error
}

// WebSearchRuntime is decrypted search config for one Run.
type WebSearchRuntime struct {
	Provider SearchProvider
	APIKey   string
}

// ScreenFunc reads the bound terminal's screen text. scrollbackLines is the
// number of scrollback rows above the live screen to include (0 = screen
// only); implementations may cap it.
type ScreenFunc func(ctx context.Context, scrollbackLines int) (string, error)

// TerminalExecSet records which approved tool calls must run inside the
// user's terminal (PTY input) instead of the exec channel. It is shared by
// pointer between the engine's approval decisions and the tool handlers.
type TerminalExecSet struct {
	mu sync.Mutex
	m  map[string]bool
}

func NewTerminalExecSet() *TerminalExecSet {
	return &TerminalExecSet{m: make(map[string]bool)}
}

func (s *TerminalExecSet) Mark(callID string) {
	if s == nil || callID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[callID] = true
}

func (s *TerminalExecSet) Has(callID string) bool {
	if s == nil || callID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[callID]
}

// Deps is injected per Run via context (not shared engine state).
type Deps struct {
	LinkID string
	Echo   bool
	SSH    SSHExecutor
	Emit   func(event.StreamEvent)

	// Screen reads the bound terminal's screen text for read_terminal. Nil
	// disables the tool (no bound session or no UI provider).
	Screen ScreenFunc
	// TerminalExec marks approved run_ssh_command calls the user chose to
	// execute inside the terminal instead of the exec channel.
	TerminalExec *TerminalExecSet

	ArmSSHExecCancel func()

	AllowedDirs []string
	WebSearch   WebSearchRuntime

	// OnPlanUpsert persists the authoritative plan snapshot ("" clears). Nil skips persist.
	OnPlanUpsert func(planJSON string) error
}

// WithDeps stores Deps on ctx for tool handlers and stream middleware.
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
	return d.LinkID != "" && d.SSH != nil
}

func (d Deps) EnableScreen() bool {
	return d.Screen != nil
}

func (d Deps) EnableFS() bool {
	return len(d.AllowedDirs) > 0
}

func (d Deps) EnableWebSearch() bool {
	p := d.WebSearch.Provider
	return ValidSearchProvider(p) && strings.TrimSpace(d.WebSearch.APIKey) != ""
}
