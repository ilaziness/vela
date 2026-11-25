package layout

// ellipsisMark is the single-character ellipsis appended to truncated text.
const ellipsisMark = "…"

// Ellipsize shortens text with a trailing ellipsis so it fits maxWidth.
// measure must be the same style the text is drawn with — pass
// w.TextWidth bound to that style.
//
// Returned text never exceeds maxWidth when measure is monotonic. The
// search is rune-safe (it never splits a multi-byte rune) but not
// grapheme-safe: a ZWJ emoji or combining sequence may be cut between
// code points. Tab titles tolerate that; it keeps the helper cheap.
//
// text is returned unchanged when it already fits, when maxWidth is not
// positive, or when measure is nil.
func Ellipsize(text string, maxWidth float32, measure func(string) float32) string {
	if measure == nil || maxWidth <= 0 || text == "" {
		return text
	}
	if measure(text) <= maxWidth {
		return text
	}
	ellipsisWidth := measure(ellipsisMark)
	if ellipsisWidth > maxWidth {
		return ""
	}
	runes := []rune(text)
	// Widest prefix that still fits beside the ellipsis. Binary search on
	// the rune count; measure is monotonic for a fixed style.
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if measure(string(runes[:mid]))+ellipsisWidth <= maxWidth {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return ellipsisMark
	}
	return string(runes[:lo]) + ellipsisMark
}
