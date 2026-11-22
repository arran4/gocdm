//go:build !windows

package x11

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func newSysProcAttrWithCred(uid, gid *int, groups []int) *syscall.SysProcAttr {
	attr := &syscall.SysProcAttr{Setsid: true}
	if uid != nil && gid != nil && os.Geteuid() == 0 {
		gids := make([]uint32, 0, len(groups)+1)
		for _, g := range groups {
			if g >= 0 {
				gids = append(gids, uint32(g))
			}
		}
		if len(gids) == 0 && *gid >= 0 {
			gids = append(gids, uint32(*gid))
		}
		attr.Credential = &syscall.Credential{
			Uid:    uint32(*uid),
			Gid:    uint32(*gid),
			Groups: gids,
		}
	}
	return attr
}

func openStartXLog(path string) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("invalid startXLog path: symlinks are not allowed")
		}
		if path != "/dev/null" && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("invalid startXLog path: not a regular file")
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to inspect log file %s: %w", path, err)
	}

	if path == "/dev/null" {
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", path, err)
		}
		return f, nil
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND|syscall.O_NOFOLLOW, 0644)
	if err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return nil, fmt.Errorf("invalid startXLog path: symlinks are not allowed")
		}
		return nil, fmt.Errorf("failed to open log file %s: %w", path, err)
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to inspect log file %s: %w", path, err)
		}
		return nil, fmt.Errorf("invalid startXLog path: not a regular file")
	}
	return f, nil
}

func signalProcessGroup(proc *os.Process, sig os.Signal) {
	if proc == nil {
		return
	}
	if sysSig, ok := sig.(syscall.Signal); ok {
		_ = syscall.Kill(-proc.Pid, sysSig)
		return
	}
	_ = proc.Signal(sig)
}
