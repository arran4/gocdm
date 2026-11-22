//go:build windows

package x11

import (
	"fmt"
	"os"
	"syscall"
)

func newSysProcAttrWithCred(uid, gid *int, groups []int) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func openStartXLog(path string) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("invalid startXLog path: symlinks are not allowed")
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to inspect log file %s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", path, err)
	}
	return f, nil
}

func signalProcessGroup(proc *os.Process, sig os.Signal) {
	if proc != nil {
		_ = proc.Signal(sig)
	}
}
