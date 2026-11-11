//go:build !windows && !darwin && !linux

package fonts

func defaultMonoFamilies() []string {
	return []string{"monospace"}
}
