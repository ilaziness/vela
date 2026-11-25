//go:build linux

package fonts

func defaultMonoFamilies() []string {
	return []string{"monospace", "DejaVu Sans Mono", "Noto Sans Mono", "Liberation Mono", "Ubuntu Mono"}
}
