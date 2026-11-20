package keybind

import "testing"

func TestNewDefaults(t *testing.T) {
	m := New(nil)
	b := m.Bindings()
	if len(b) != 6 {
		t.Fatalf("bindings = %d, want 6", len(b))
	}
	if b[PanelQuickConnect] != "CmdOrCtrl+B" {
		t.Fatalf("panel.quickConnect = %q", b[PanelQuickConnect])
	}
	for _, gone := range []string{"terminal.addToChat", "terminal.clear", "panel.bookmarkConnect", "panel.command", "panel.bookmarks", "panel.tools", "panel.settings"} {
		if _, ok := b[gone]; ok {
			t.Fatalf("removed action %q still present", gone)
		}
	}
}

func TestSetConflictAndOnChange(t *testing.T) {
	m := New(nil)
	fired := 0
	m.SetOnChange(func() { fired++ })

	// Bind panel.quickConnect to the same chord as panel.ai's default.
	if err := m.Set(PanelQuickConnect, "CmdOrCtrl+Shift+J"); err == nil {
		t.Fatal("expected conflict error")
	}
	if fired != 0 {
		t.Fatalf("onChange fired %d times on failed set", fired)
	}

	if err := m.Set(PanelQuickConnect, "CmdOrCtrl+Shift+T"); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if fired != 1 {
		t.Fatalf("onChange fired %d times, want 1", fired)
	}
	if got := m.Bindings()[PanelQuickConnect]; got != "CmdOrCtrl+Shift+T" {
		t.Fatalf("panel.quickConnect = %q", got)
	}

	// Reset restores the default and notifies again.
	if err := m.Reset(PanelQuickConnect); err != nil {
		t.Fatalf("reset failed: %v", err)
	}
	if fired != 2 {
		t.Fatalf("onChange fired %d times, want 2", fired)
	}
	if got := m.Bindings()[PanelQuickConnect]; got != "CmdOrCtrl+B" {
		t.Fatalf("panel.quickConnect after reset = %q", got)
	}
}

func TestSetUnknownAction(t *testing.T) {
	m := New(nil)
	if err := m.Set("nope.action", "CmdOrCtrl+X"); err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestSetRejectsUnknownKeyName(t *testing.T) {
	m := New(nil)
	if err := m.Set(PanelQuickConnect, "CmdOrCtrl+NotAKey"); err == nil {
		t.Fatal("expected error for multi-char unknown key")
	}
	if err := m.Set(PanelQuickConnect, "CmdOrCtrl+."); err != nil {
		t.Fatalf("single-char punctuation should be allowed: %v", err)
	}
}
