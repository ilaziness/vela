package app

import (
	"testing"

	"github.com/go-gui-org/go-term/term"
)

func TestCursorStyleOf(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   string
		want term.CursorStyle
	}{
		{"", term.CursorStyleBlock},
		{"block", term.CursorStyleBlock},
		{"underline", term.CursorStyleUnderline},
		{" Bar ", term.CursorStyleBar},
		{"wedge", term.CursorStyleBlock},
	} {
		if got := cursorStyleOf(tc.in); got != tc.want {
			t.Fatalf("cursorStyleOf(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}
