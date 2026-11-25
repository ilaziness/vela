//go:build !windows

package updater

import (
	"fmt"
	"syscall"
	"time"
)

// waitForExit polls until pid is gone. Unix cannot waitpid a non-child.
func waitForExit(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH || err == syscall.EINVAL {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for pid %d", pid)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
