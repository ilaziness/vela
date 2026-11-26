package app

import (
	"testing"

	"github.com/ilaziness/vela/internal/keybind"
)

func TestKeybindKeys(t *testing.T) {
	c := NewController(nil, nil, nil)
	if got := c.KeybindKeys(keybind.TerminalCopy); got != "" {
		t.Fatalf("KeybindKeys without manager = %q, want empty", got)
	}

	c.SetKeybind(keybind.New(nil))
	if got := c.KeybindKeys(keybind.TerminalCopy); got == "" {
		t.Fatal("KeybindKeys(TerminalCopy) is empty, want a display shortcut")
	}
	if got := c.KeybindKeys("no.such.action"); got != "" {
		t.Fatalf("KeybindKeys(unknown) = %q, want empty", got)
	}
}
