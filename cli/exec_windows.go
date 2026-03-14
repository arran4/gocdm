//go:build windows

package cli

import (
	"os"
	"os/exec"
)

func ExecProgram(binary string, args []string, env []string) error {
	cmd := exec.Command(binary)

	// Ensure args matches exec.Command behavior (args[0] is typically the binary name)
	// On Windows, args is sometimes passed as just the arguments without the binary name in the first place,
	// but according to cli.go `ExecProgramFn(binary, append([]string{bin}, args...), env)`,
	// args[0] is indeed the binary name. `exec.Command`'s Cmd.Args includes the command name.
	if len(args) > 0 {
		cmd.Args = args
	}

	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		return err
	}

	// Exec usually replaces the process. To simulate this, we exit when the child finishes.
	os.Exit(0)
	return nil
}
