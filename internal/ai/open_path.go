package ai

import "github.com/ilaziness/vela/internal/system"

// OpenPath opens a file or directory with the OS default application.
func OpenPath(path string) error {
	return system.OpenPath(path)
}
