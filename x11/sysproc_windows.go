//go:build windows

package x11

import "syscall"

func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
