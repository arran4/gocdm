//go:build windows

package cli

import "fmt"

func ExecProgram(binary string, args []string, env []string) error {
	return fmt.Errorf("exec is not supported on windows")
}
