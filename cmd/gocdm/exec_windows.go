//go:build windows

package main

import "fmt"

func execProgram(binary string, args []string, env []string) error {
	return fmt.Errorf("exec is not supported on windows")
}
