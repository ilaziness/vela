package session

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/keybind"
)

func menuTextByID(items []gui.MenuItemCfg, id string) (string, bool) {
	for _, it := range items {
		if it.ID == id {
			return it.Text, true
		}
	}
	return "", false
}

func TestTermMenuItemsShortcutHints(t *testing.T) {
	keys := func(action string) string { return "Ctrl+Shift+" + action }
	items := termMenuItems(nil, "s1", nil, false, false, keys)

	for _, tc := range []struct {
		id, action string
	}{
		{"copy", keybind.TerminalCopy},
		{"paste", keybind.TerminalPaste},
		{"find", keybind.TerminalFind},
		{"record", keybind.TerminalToggleLog},
	} {
		text, ok := menuTextByID(items, tc.id)
		if !ok {
			t.Fatalf("missing menu item %q", tc.id)
		}
		if !strings.Contains(text, "(Ctrl+Shift+"+tc.action+")") {
			t.Fatalf("item %q text %q lacks shortcut hint", tc.id, text)
		}
	}
}

func TestTermMenuItemsNoHintWithoutKeys(t *testing.T) {
	items := termMenuItems(nil, "s1", nil, false, false, func(string) string { return "" })
	for _, it := range items {
		if it.Separator {
			continue
		}
		if strings.Contains(it.Text, "(") {
			t.Fatalf("item %q text %q has unexpected hint", it.ID, it.Text)
		}
	}
}

func TestTermMenuItemsReplayOmitsPasteAndRecord(t *testing.T) {
	items := termMenuItems(nil, "s1", nil, true, false, func(string) string { return "Ctrl+X" })
	for _, id := range []string{"paste", "record"} {
		if _, ok := menuTextByID(items, id); ok {
			t.Fatalf("replay menu should not contain %q", id)
		}
	}
	if _, ok := menuTextByID(items, "copy"); !ok {
		t.Fatal("replay menu should still contain copy")
	}
	if _, ok := menuTextByID(items, "find"); !ok {
		t.Fatal("replay menu should still contain find")
	}
}
