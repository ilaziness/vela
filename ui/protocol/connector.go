// Package protocol defines desktop-side protocol plugins for session connect UI.
package protocol

import (
	"context"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/termio"
)

// Draft is a protocol-agnostic connect form snapshot.
type Draft struct {
	Protocol termio.Protocol
	Payload  any
}

// Bus lets a form trigger connect/test without knowing the controller.
type Bus interface {
	Connect(draft Draft)
	Test(draft Draft)
	SetStatus(msg string)
	Window() *gui.Window
	// Busy reports an in-flight connect/test so forms can disable
	// actions and show a loading face. op is "connect", "test", or "".
	Busy() (busy bool, op string)
}

// Connector dials a protocol. Implementations must be safe for background calls.
type Connector interface {
	Protocol() termio.Protocol
	Connect(ctx context.Context, draft Draft) (termio.Session, error)
	Test(ctx context.Context, draft Draft) error
}

// FormFactory builds the connect form for one protocol.
type FormFactory func(bus Bus, draft *Draft) gui.View

// Entry is one registered protocol plugin.
type Entry struct {
	Protocol termio.Protocol
	Label    string
	Enabled  bool
	Connect  Connector
	Form     FormFactory
	NewDraft func() Draft
}

// Registry maps protocols to plugins.
type Registry struct {
	order []termio.Protocol
	by    map[termio.Protocol]Entry
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{by: make(map[termio.Protocol]Entry)}
}

// Register adds or replaces an entry.
func (r *Registry) Register(e Entry) {
	if r == nil || e.Protocol == "" {
		return
	}
	if _, ok := r.by[e.Protocol]; !ok {
		r.order = append(r.order, e.Protocol)
	}
	r.by[e.Protocol] = e
}

// Get returns a registered entry.
func (r *Registry) Get(p termio.Protocol) (Entry, bool) {
	if r == nil {
		return Entry{}, false
	}
	e, ok := r.by[p]
	return e, ok
}

// Entries returns registration order.
func (r *Registry) Entries() []Entry {
	if r == nil {
		return nil
	}
	out := make([]Entry, 0, len(r.order))
	for _, p := range r.order {
		if e, ok := r.by[p]; ok {
			out = append(out, e)
		}
	}
	return out
}
