package keybind

import (
	"strings"
	"sync"

	"github.com/ilaziness/vela/internal/config"
	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/internal/usererr"
)

// Manager merges defaults with config overrides. Global hotkey dispatch
// can be wired later; this package owns persistence for the settings UI.
type Manager struct {
	mu      sync.Mutex
	store   *config.Store
	defs    map[string]def
	current map[string]string // action -> logical
}

// New builds a manager and loads overrides from store.
func New(store *config.Store) *Manager {
	m := &Manager{
		store:   store,
		defs:    defMap(),
		current: make(map[string]string),
	}
	for _, d := range builtin() {
		m.current[d.action] = d.logical
	}
	if store != nil && store.Config != nil {
		for action, logical := range store.Config.Keybindings {
			if _, ok := m.defs[action]; !ok {
				continue
			}
			logical = strings.TrimSpace(logical)
			if logical == "" {
				continue
			}
			if err := validateLogical(logical); err != nil {
				continue
			}
			m.current[action] = canonicalizeLogical(logical)
		}
	}
	return m
}

// List returns localized binding rows.
func (m *Manager) List() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	defs := builtin()
	out := make([]Info, 0, len(defs))
	for _, d := range defs {
		logical := m.current[d.action]
		if logical == "" {
			logical = d.logical
		}
		out = append(out, localized(d, logical))
	}
	return out
}

// Set updates one binding. Empty logical resets to default.
func (m *Manager) Set(action, logical string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := m.defs[action]
	if !ok {
		return usererr.Coded("keybinding.unknown_action", "Unknown shortcut action: %s", action)
	}

	logical = strings.TrimSpace(logical)
	oldLogical := m.current[action]
	if oldLogical == "" {
		oldLogical = d.logical
	}

	clearOverride := logical == ""
	var newLogical string
	if clearOverride {
		newLogical = d.logical
	} else {
		if err := validateLogical(logical); err != nil {
			return err
		}
		newLogical = canonicalizeLogical(logical)
		if normalizeAccel(newLogical) == normalizeAccel(d.logical) {
			clearOverride = true
			newLogical = d.logical
		}
	}

	if normalizeAccel(newLogical) == normalizeAccel(oldLogical) {
		if clearOverride {
			return m.writeOverrideLocked(action, "")
		}
		return nil
	}

	newNorm := normalizeAccel(newLogical)
	for act, cur := range m.current {
		if act == action {
			continue
		}
		if normalizeAccel(cur) == newNorm {
			label := act
			if xd, ok := m.defs[act]; ok {
				label = i18n.T(xd.labelKey)
			}
			return usererr.Coded("keybinding.conflict", "Shortcut conflicts with \"%s\"", label)
		}
	}

	m.current[action] = newLogical
	if clearOverride {
		return m.writeOverrideLocked(action, "")
	}
	return m.writeOverrideLocked(action, newLogical)
}

// Reset restores one action to its default.
func (m *Manager) Reset(action string) error {
	return m.Set(action, "")
}

// ResetAll clears all overrides.
func (m *Manager) ResetAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store != nil && m.store.Config != nil {
		prev := m.store.Config.Keybindings
		m.store.Config.Keybindings = nil
		if err := m.store.Save(); err != nil {
			m.store.Config.Keybindings = prev
			return usererr.Coded("keybinding.save_failed", "Failed to save shortcuts: %s", err.Error())
		}
	}
	for action := range m.current {
		m.current[action] = m.defs[action].logical
	}
	return nil
}

func (m *Manager) writeOverrideLocked(action, logical string) error {
	if m.store == nil || m.store.Config == nil {
		return nil
	}
	prev := m.store.Config.Keybindings
	kb := make(map[string]string, len(prev)+1)
	for k, v := range prev {
		kb[k] = v
	}
	if logical == "" {
		delete(kb, action)
	} else {
		kb[action] = logical
	}
	if len(kb) == 0 {
		kb = nil
	}
	m.store.Config.Keybindings = kb
	if err := m.store.Save(); err != nil {
		m.store.Config.Keybindings = prev
		return usererr.Coded("keybinding.save_failed", "Failed to save shortcuts: %s", err.Error())
	}
	return nil
}
