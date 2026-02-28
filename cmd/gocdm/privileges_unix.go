//go:build !windows

package main

import "syscall"

func setGroupID(gid int) error {
	return syscall.Setgid(gid)
}

func setUserID(uid int) error {
	return syscall.Setuid(uid)
}
