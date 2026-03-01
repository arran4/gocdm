//go:build !windows

package cli

import "syscall"

func ExecProgram(binary string, args []string, env []string) error {
	return syscall.Exec(binary, args, env)
}
