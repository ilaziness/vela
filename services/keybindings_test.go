package services

import (
	"runtime"
	"testing"

	"github.com/ilaziness/vela/internal/config"
)

func TestNormalizeAccel(t *testing.T) {
	got := normalizeAccel("CmdOrCtrl+Shift+C")
	wantMod := "Ctrl"
	if runtime.GOOS == "darwin" {
		wantMod = "Cmd"
	}
	want := wantMod + "+Shift+C"
	if got != want {
		t.Fatalf("normalizeAccel(CmdOrCtrl+Shift+C) = %q, want %q", got, want)
	}

	got = normalizeAccel("CmdOrCtrl+F")
	want = wantMod + "+F"
	if got != want {
		t.Fatalf("normalizeAccel(CmdOrCtrl+F) = %q, want %q", got, want)
	}

	got = normalizeAccel("CmdOrCtrl+,")
	want = wantMod + "+,"
	if got != want {
		t.Fatalf("normalizeAccel(CmdOrCtrl+,) = %q, want %q", got, want)
	}
}

func TestNormalizeAccelCmdOrCtrlEqualsCtrl(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Ctrl vs CmdOrCtrl equivalence is for non-darwin")
	}
	a := normalizeAccel("CmdOrCtrl+B")
	b := normalizeAccel("Ctrl+B")
	if a != b {
		t.Fatalf("CmdOrCtrl+B (%q) != Ctrl+B (%q)", a, b)
	}
}

func TestAccelAliasesWindowsComma(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("OEM_COMMA alias is Windows-only")
	}
	aliases := accelAliases("Ctrl+,")
	if len(aliases) != 1 || aliases[0] != "Ctrl+OEM_COMMA" {
		t.Fatalf("accelAliases(Ctrl+,) = %v, want [Ctrl+OEM_COMMA]", aliases)
	}
}

func TestDisplayKeys(t *testing.T) {
	got := displayKeys("CmdOrCtrl+Shift+C")
	if runtime.GOOS == "darwin" {
		if got != "⌘⇧C" {
			t.Fatalf("displayKeys on darwin = %q, want ⌘⇧C", got)
		}
		return
	}
	if got != "Ctrl+Shift+C" {
		t.Fatalf("displayKeys = %q, want Ctrl+Shift+C", got)
	}
}

func TestCanonicalizeLogical(t *testing.T) {
	got := canonicalizeLogical("Ctrl+Shift+b")
	if got != "CmdOrCtrl+Shift+B" {
		t.Fatalf("canonicalizeLogical = %q, want CmdOrCtrl+Shift+B", got)
	}
	got = canonicalizeLogical("CmdOrCtrl+,")
	if got != "CmdOrCtrl+," {
		t.Fatalf("canonicalizeLogical comma = %q", got)
	}
}

func TestValidateLogical(t *testing.T) {
	if err := validateLogical("CmdOrCtrl+B"); err != nil {
		t.Fatalf("valid chord: %v", err)
	}
	if err := validateLogical("F5"); err != nil {
		t.Fatalf("F5 should be allowed: %v", err)
	}
	if err := validateLogical("Alt+F"); err != nil {
		t.Fatalf("Alt chord should be allowed: %v", err)
	}
	if err := validateLogical("B"); err == nil {
		t.Fatal("bare letter should be rejected")
	}
	if err := validateLogical("Shift+B"); err == nil {
		t.Fatal("Shift-only chord should be rejected")
	}
	if err := validateLogical("Escape"); err == nil {
		t.Fatal("Escape should be rejected")
	}
}

func TestSanitizeOverrideSwap(t *testing.T) {
	store := &config.Store{Config: config.Default()}
	store.Config.Keybindings = map[string]string{
		"terminal.copy":  "CmdOrCtrl+Shift+V",
		"terminal.paste": "CmdOrCtrl+Shift+C",
	}
	m := NewKeyBindingManager(nil, nil, store, nil)
	m.mu.Lock()
	clean := m.sanitizeOverridesLocked(store.Config.Keybindings)
	m.mu.Unlock()
	if clean["terminal.copy"] != "CmdOrCtrl+Shift+V" || clean["terminal.paste"] != "CmdOrCtrl+Shift+C" {
		t.Fatalf("swap overrides should both apply, got %#v", clean)
	}
}

func TestSanitizeOverrides(t *testing.T) {
	store := &config.Store{Config: config.Default()}
	store.Config.Keybindings = map[string]string{
		"panel.bookmarkConnect": "CmdOrCtrl+Shift+X",
		"unknown.action":        "CmdOrCtrl+Z",
		"terminal.find":         "CmdOrCtrl+F",       // same as default — drop
		"terminal.copy":         "B",                 // invalid
		"terminal.paste":        "CmdOrCtrl+Shift+C", // conflicts with copy default
	}
	m := NewKeyBindingManager(nil, nil, store, nil)
	m.mu.Lock()
	clean := m.sanitizeOverridesLocked(store.Config.Keybindings)
	m.mu.Unlock()

	if _, ok := clean["panel.bookmarkConnect"]; !ok {
		t.Fatalf("expected valid override kept, got %#v", clean)
	}
	if _, ok := clean["unknown.action"]; ok {
		t.Fatal("unknown action should be dropped")
	}
	if _, ok := clean["terminal.find"]; ok {
		t.Fatal("redundant default override should be dropped")
	}
	if _, ok := clean["terminal.copy"]; ok {
		t.Fatal("invalid override should be dropped")
	}
	if _, ok := clean["terminal.paste"]; ok {
		t.Fatal("conflicting override should be dropped")
	}
}

func TestSetConflictAndReset(t *testing.T) {
	store := &config.Store{Config: config.Default()}
	m := NewKeyBindingManager(nil, nil, store, nil)
	m.Register()

	err := m.Set(ShortcutPanelBookmarkConnect, "CmdOrCtrl+Shift+C")
	if err == nil {
		t.Fatal("expected conflict with terminal.copy")
	}

	if err := m.Set(ShortcutPanelBookmarkConnect, "CmdOrCtrl+Shift+X"); err != nil {
		t.Fatalf("set custom: %v", err)
	}
	if store.Config.Keybindings[ShortcutPanelBookmarkConnect] != "CmdOrCtrl+Shift+X" {
		t.Fatalf("override not persisted: %#v", store.Config.Keybindings)
	}

	list := m.List()
	var found bool
	for _, info := range list {
		if info.Action == ShortcutPanelBookmarkConnect {
			found = true
			if !info.Custom || info.Logical != "CmdOrCtrl+Shift+X" {
				t.Fatalf("list info = %#v", info)
			}
		}
	}
	if !found {
		t.Fatal("bookmark connect missing from list")
	}

	// set to default clears override
	if err := m.Set(ShortcutPanelBookmarkConnect, "CmdOrCtrl+B"); err != nil {
		t.Fatalf("set default: %v", err)
	}
	if _, ok := store.Config.Keybindings[ShortcutPanelBookmarkConnect]; ok {
		t.Fatalf("default should clear override: %#v", store.Config.Keybindings)
	}

	if err := m.Set(ShortcutPanelBookmarkConnect, "CmdOrCtrl+Shift+J"); err != nil {
		t.Fatal(err)
	}
	if err := m.ResetAll(); err != nil {
		t.Fatal(err)
	}
	if len(store.Config.Keybindings) > 0 {
		t.Fatalf("reset should clear: %#v", store.Config.Keybindings)
	}
}

func TestSetEmptyResets(t *testing.T) {
	store := &config.Store{Config: config.Default()}
	m := NewKeyBindingManager(nil, nil, store, nil)
	m.Register()
	_ = m.Set(ShortcutPanelAI, "CmdOrCtrl+Shift+Z")
	if err := m.Set(ShortcutPanelAI, ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Config.Keybindings[ShortcutPanelAI]; ok {
		t.Fatal("empty logical should clear override")
	}
}
