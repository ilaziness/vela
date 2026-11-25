package term

// ScreenText returns the terminal's text content as plain UTF-8: the live
// screen plus up to scrollbackTail scrollback rows above it, in content
// order. Soft-wrapped rows are joined into logical lines and trailing blanks
// are trimmed — the same rules a linear selection copies by (see
// grid.rowsText). scrollbackTail <= 0 returns only the live screen.
//
// While the alt screen is active the scrollback buffer still belongs to the
// main screen, so scrollbackTail is ignored and only the alt screen's own
// rows are returned.
//
// Safe to call from any goroutine; returns "" before the first layout.
func (t *Term) ScreenText(scrollbackTail int) string {
	t.grid.Mu.Lock()
	defer t.grid.Mu.Unlock()

	g := t.grid
	sb := g.Scrollback.Len()
	if g.Rows <= 0 || g.Cols <= 0 {
		return ""
	}
	r0 := sb
	if !g.AltActive && scrollbackTail > 0 {
		r0 = max(0, sb-scrollbackTail)
	}
	return g.rowsText(r0, sb+g.Rows-1)
}
