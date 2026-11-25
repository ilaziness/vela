package term

import (
	"strings"
	"testing"
)

func feedScreenText(g *grid, s string) {
	for _, r := range s {
		if r == '\n' {
			g.CarriageReturn()
			g.Newline()
			continue
		}
		g.Put(r)
	}
}

func TestScreenText_LiveScreenOnly(t *testing.T) {
	tm, _ := newTestTermCapture() // 4 rows x 8 cols
	feedScreenText(tm.grid, "aaa\nbbb\nccc\nddd\neee\nfff")

	got := tm.ScreenText(0)
	if !strings.Contains(got, "ddd") || !strings.Contains(got, "fff") {
		t.Fatalf("screen text missing tail rows: %q", got)
	}
	if strings.Contains(got, "aaa") || strings.Contains(got, "bbb") {
		t.Fatalf("scrollback leaked into screen-only read: %q", got)
	}
}

func TestScreenText_ScrollbackTail(t *testing.T) {
	tm, _ := newTestTermCapture()
	tm.grid.ScrollbackCap = 100
	feedScreenText(tm.grid, "aaa\nbbb\nccc\nddd\neee\nfff")

	got := tm.ScreenText(100)
	for _, want := range []string{"aaa", "bbb", "ccc", "ddd", "eee", "fff"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestScreenText_AltActiveIgnoresScrollback(t *testing.T) {
	tm, _ := newTestTermCapture()
	feedScreenText(tm.grid, "aaa\nbbb\nccc\nddd\neee\nfff")
	tm.grid.AltActive = true

	got := tm.ScreenText(100)
	if strings.Contains(got, "aaa") || strings.Contains(got, "bbb") {
		t.Fatalf("alt screen must not expose main scrollback: %q", got)
	}
	if !strings.Contains(got, "fff") {
		t.Fatalf("alt screen lost live rows: %q", got)
	}
}
