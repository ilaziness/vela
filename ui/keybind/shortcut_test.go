package keybind

import (
	"runtime"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestShortcutFor(t *testing.T) {
	sc, ok := ShortcutFor("CmdOrCtrl+Shift+C")
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if sc.Key != gui.KeyC {
		t.Fatalf("key = %v, want KeyC", sc.Key)
	}
	wantPrimary := gui.ModCtrl
	if runtime.GOOS == "darwin" {
		wantPrimary = gui.ModSuper
	}
	if sc.Modifiers != wantPrimary|gui.ModShift {
		t.Fatalf("modifiers = %v, want %v", sc.Modifiers, wantPrimary|gui.ModShift)
	}
}

func TestShortcutForComma(t *testing.T) {
	sc, ok := ShortcutFor("CmdOrCtrl+,")
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if sc.Key != gui.KeyComma {
		t.Fatalf("key = %v, want KeyComma", sc.Key)
	}
}

func TestShortcutForInvalid(t *testing.T) {
	for _, s := range []string{"", "CmdOrCtrl+", "CmdOrCtrl+NoSuchKey", "CmdOrCtrl+Hyper+X"} {
		if _, ok := ShortcutFor(s); ok {
			t.Fatalf("ShortcutFor(%q) unexpectedly succeeded", s)
		}
	}
}

func TestEventLogicalRoundTrip(t *testing.T) {
	e := &gui.Event{
		Type:      gui.EventKeyDown,
		KeyCode:   gui.KeyC,
		Modifiers: gui.ModCtrl | gui.ModShift,
	}
	logical, ok := EventLogical(e)
	if !ok {
		t.Fatal("expected logical conversion")
	}
	if logical != "CmdOrCtrl+Shift+C" {
		t.Fatalf("logical = %q", logical)
	}
	sc, ok := ShortcutFor(logical)
	if !ok || sc.Key != gui.KeyC {
		t.Fatalf("round trip failed: %+v ok=%v", sc, ok)
	}
}

func TestEventLogicalIgnoresModifiersAndEscape(t *testing.T) {
	for _, e := range []*gui.Event{
		{Type: gui.EventKeyDown, KeyCode: gui.KeyLeftShift, Modifiers: gui.ModShift},
		{Type: gui.EventKeyDown, KeyCode: gui.KeyEscape},
		nil,
	} {
		if _, ok := EventLogical(e); ok {
			t.Fatalf("EventLogical(%+v) unexpectedly succeeded", e)
		}
	}
}

func TestEventLogicalAltOrder(t *testing.T) {
	e := &gui.Event{
		Type:      gui.EventKeyDown,
		KeyCode:   gui.KeyF5,
		Modifiers: gui.ModAlt,
	}
	logical, ok := EventLogical(e)
	if !ok || logical != "Alt+F5" {
		t.Fatalf("logical = %q ok=%v", logical, ok)
	}
}

func TestShortcutForPunctuation(t *testing.T) {
	sc, ok := ShortcutFor("CmdOrCtrl+.")
	if !ok {
		t.Fatal("expected punctuation parse to succeed")
	}
	if sc.Key != gui.KeyPeriod {
		t.Fatalf("key = %v, want KeyPeriod", sc.Key)
	}
}
