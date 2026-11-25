//go:build !windows

package updater

import "syscall"

func detachProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
