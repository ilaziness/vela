//go:build windows

package fonts

// System mono first (go-glyph maps "monospace" → Consolas). Named faces are
// only used when config lists them or when probing a stack; CJK still falls
// back to YaHei via script fallbacks, not as the primary terminal face.
func defaultMonoFamilies() []string {
	return []string{"monospace", "Consolas", "Cascadia Mono", "Courier New"}
}
