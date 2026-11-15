package config

import "testing"

func TestTerminalConfigNormalize(t *testing.T) {
	t.Parallel()

	c := TerminalConfig{Scrollback: 0, FontSize: 0, Font: ""}
	c.Normalize()
	if c.Scrollback != defaultTerminalScrollback {
		t.Fatalf("scrollback=%d", c.Scrollback)
	}
	if c.FontSize != 14 {
		t.Fatalf("fontSize=%d", c.FontSize)
	}
	if c.Font != defaultTerminalFont {
		t.Fatalf("font=%q want %q", c.Font, defaultTerminalFont)
	}
	if c.Theme != defaultTerminalTheme {
		t.Fatalf("theme=%q", c.Theme)
	}
	if c.CursorStyle != "block" {
		t.Fatalf("cursorStyle=%q", c.CursorStyle)
	}
	if c.CursorBlink || c.CursorLock || c.RecordInput {
		t.Fatalf("bool defaults mutated: %+v", c)
	}
	if c.RecordDir != "" {
		t.Fatalf("recordDir=%q", c.RecordDir)
	}

	c = TerminalConfig{Scrollback: 50, FontSize: 14, Font: "Consolas"}
	c.Normalize()
	if c.Scrollback != minTerminalScrollback {
		t.Fatalf("scrollback clamp low=%d", c.Scrollback)
	}

	c = TerminalConfig{Scrollback: 999999, FontSize: 14, Font: "Consolas"}
	c.Normalize()
	if c.Scrollback != maxTerminalScrollback {
		t.Fatalf("scrollback clamp high=%d", c.Scrollback)
	}

	c = TerminalConfig{Font: "Consolas", FontSize: 14, Scrollback: 5000}
	c.Normalize()
	if c.CursorBlink || c.CursorLock || c.RecordInput {
		t.Fatalf("bools mutated: %+v", c)
	}

	c = TerminalConfig{Theme: "   ", Font: "Consolas", FontSize: 14, Scrollback: 5000}
	c.Normalize()
	if c.Theme != defaultTerminalTheme {
		t.Fatalf("blank theme=%q", c.Theme)
	}

	c = TerminalConfig{Theme: " Dracula ", Font: "Consolas", FontSize: 14, Scrollback: 5000}
	c.Normalize()
	if c.Theme != "Dracula" {
		t.Fatalf("theme=%q", c.Theme)
	}

	if Default().Terminal.Theme != defaultTerminalTheme {
		t.Fatalf("default theme = %q", Default().Terminal.Theme)
	}
}

func TestTerminalConfigNormalizeCursorStyle(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in, want string
	}{
		{"", "block"},
		{"block", "block"},
		{" Underline ", "underline"},
		{"BAR", "bar"},
		{"wedge", "block"},
		{"auto", "block"},
	} {
		c := TerminalConfig{Font: "Consolas", FontSize: 14, Scrollback: 5000, CursorStyle: tc.in}
		c.Normalize()
		if c.CursorStyle != tc.want {
			t.Fatalf("cursorStyle(%q)=%q want %q", tc.in, c.CursorStyle, tc.want)
		}
	}
}

func TestTerminalConfigNormalizeRecordDir(t *testing.T) {
	t.Parallel()

	c := TerminalConfig{Font: "Consolas", FontSize: 14, Scrollback: 5000, RecordDir: "  /tmp/rec  "}
	c.Normalize()
	if c.RecordDir != "/tmp/rec" {
		t.Fatalf("recordDir=%q", c.RecordDir)
	}
}
