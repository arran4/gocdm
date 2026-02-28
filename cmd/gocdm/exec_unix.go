//go:build !windows

package main

import "syscall"

func execProgram(binary string, args []string, env []string) error {
	return syscall.Exec(binary, args, env)
}
