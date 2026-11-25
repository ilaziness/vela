package app

import (
	"slices"
	"testing"
)

func TestTermThemeItems(t *testing.T) {
	c := NewController(nil, nil, nil)
	items := c.TermThemeItems()
	if len(items) == 0 {
		t.Fatal("no theme items")
	}
	if items[0] != "Default" {
		t.Fatalf("first item = %q, want Default", items[0])
	}
	if len(items) <= 600 {
		t.Fatalf("items = %d, want bundled themes included", len(items))
	}
	for _, want := range []string{"Dracula", "iTerm2 Solarized Light"} {
		if !slices.Contains(items, want) {
			t.Errorf("items missing %q", want)
		}
	}
}

func TestTermThemeName(t *testing.T) {
	c := NewController(nil, nil, nil)
	cases := []struct {
		in   string
		want string
	}{
		{"", "Default"},
		{"default", "Default"},
		{"Dracula", "Dracula"},
		{"dracula", "Dracula"},
		{"ITERM2 SOLARIZED LIGHT", "iTerm2 Solarized Light"},
		{"bogus", "Default"},
		{"  ", "Default"},
	}
	for _, tc := range cases {
		if got := c.TermThemeName(tc.in); got != tc.want {
			t.Errorf("TermThemeName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSetTermTheme(t *testing.T) {
	c := NewController(nil, nil, nil)
	if got := c.termTheme.Name; got != "Default" {
		t.Fatalf("initial theme = %q", got)
	}

	c.SetTermTheme("dracula")
	if got := c.termTheme.Name; got != "Dracula" {
		t.Fatalf("theme = %q, want Dracula", got)
	}

	c.SetTermTheme("bogus")
	if got := c.termTheme.Name; got != "Default" {
		t.Fatalf("fallback theme = %q", got)
	}
}

func TestTermThemeZeroController(t *testing.T) {
	c := &Controller{}
	if got := c.TermThemeName("dracula"); got != "Dracula" {
		t.Fatalf("TermThemeName = %q", got)
	}
	c.SetTermTheme("bogus")
	if got := c.termTheme.Name; got != "Default" {
		t.Fatalf("zero-value fallback = %q", got)
	}
}
