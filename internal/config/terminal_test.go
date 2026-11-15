package config

import "testing"

func TestTerminalConfigNormalize(t *testing.T) {
	t.Parallel()

	c := TerminalConfig{Scrollback: 0, FontSize: 0, LineHeight: 0, Font: ""}
	c.Normalize()
	if c.Scrollback != defaultTerminalScrollback {
		t.Fatalf("scrollback=%d", c.Scrollback)
	}
	if c.FontSize != 14 {
		t.Fatalf("fontSize=%d", c.FontSize)
	}
	if c.LineHeight != 1 {
		t.Fatalf("lineHeight=%v", c.LineHeight)
	}
	if c.Font != defaultTerminalFont {
		t.Fatalf("font=%q want %q", c.Font, defaultTerminalFont)
	}
	if c.Theme != defaultTerminalTheme {
		t.Fatalf("theme=%q", c.Theme)
	}

	c = TerminalConfig{Scrollback: 50, FontSize: 14, LineHeight: 1, Font: "Consolas"}
	c.Normalize()
	if c.Scrollback != minTerminalScrollback {
		t.Fatalf("scrollback clamp low=%d", c.Scrollback)
	}

	c = TerminalConfig{Scrollback: 999999, FontSize: 14, LineHeight: 1, Font: "Consolas"}
	c.Normalize()
	if c.Scrollback != maxTerminalScrollback {
		t.Fatalf("scrollback clamp high=%d", c.Scrollback)
	}

	// Normalize must not flip user-disabled bell back on.
	c = TerminalConfig{
		Font: "Consolas", FontSize: 14, LineHeight: 1,
		Scrollback: 5000, Bell: false, CopyOnSelect: true, RightClickPaste: true,
	}
	c.Normalize()
	if c.Bell || !c.CopyOnSelect || !c.RightClickPaste {
		t.Fatalf("bools mutated: %+v", c)
	}

	c = TerminalConfig{Theme: "   ", Font: "Consolas", FontSize: 14, LineHeight: 1, Scrollback: 5000}
	c.Normalize()
	if c.Theme != defaultTerminalTheme {
		t.Fatalf("blank theme=%q", c.Theme)
	}

	c = TerminalConfig{Theme: " Dracula ", Font: "Consolas", FontSize: 14, LineHeight: 1, Scrollback: 5000}
	c.Normalize()
	if c.Theme != "Dracula" {
		t.Fatalf("theme=%q", c.Theme)
	}

	if Default().Terminal.Theme != defaultTerminalTheme {
		t.Fatalf("default theme = %q", Default().Terminal.Theme)
	}
}
