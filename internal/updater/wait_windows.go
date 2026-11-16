//go:build windows

package updater

import (
	"fmt"
	"os"
	"time"
)

// waitForExit waits on the process handle opened by os.FindProcess.
func waitForExit(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return errProcessGone
	}
	done := make(chan error, 1)
	go func() { _, err := p.Wait(); done <- err }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for pid %d", pid)
	}
}
