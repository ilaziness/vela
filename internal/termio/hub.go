package termio

import (
	"sync"
)

// Hub multiplexes active terminal sessions. It is not the SSH client store.
type Hub struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewHub() *Hub {
	return &Hub{sessions: make(map[string]Session)}
}

// Register stores s by ID. Replaces any previous entry with the same ID.
func (h *Hub) Register(s Session) {
	if h == nil || s == nil {
		return
	}
	id := s.ID()
	if id == "" {
		return
	}
	h.mu.Lock()
	h.sessions[id] = s
	h.mu.Unlock()
}

// Get returns a registered session.
func (h *Hub) Get(id string) (Session, bool) {
	if h == nil || id == "" {
		return nil, false
	}
	h.mu.RLock()
	s, ok := h.sessions[id]
	h.mu.RUnlock()
	return s, ok
}

// Unregister removes id without closing the session.
func (h *Hub) Unregister(id string) {
	if h == nil || id == "" {
		return
	}
	h.mu.Lock()
	delete(h.sessions, id)
	h.mu.Unlock()
}

// Clear drops registrations without closing sessions (engines already tore them down).
func (h *Hub) Clear() {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.sessions = make(map[string]Session)
	h.mu.Unlock()
}

// Active returns listing info for all sessions.
func (h *Hub) Active() []Info {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Info, 0, len(h.sessions))
	for _, s := range h.sessions {
		if s == nil {
			continue
		}
		out = append(out, Info{
			ID:       s.ID(),
			Protocol: s.Protocol(),
			Label:    s.Label(),
		})
	}
	return out
}
