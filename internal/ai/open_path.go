package ai

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenPath opens a file or directory with the OS default application.
func OpenPath(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
