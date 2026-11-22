//go:build windows

package x11

import "syscall"

func newSysProcAttrWithCred(uid, gid *int, groups []int) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
