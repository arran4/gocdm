//go:build windows

package cli

import (
	"errors"
	"os"
	"os/exec"

	"github.com/arran4/gocdm/config"
)

func runSupervisedSession(spec SupervisedSessionSpec) (int, error) {
	args := spec.Args
	if len(args) == 0 {
		args = []string{spec.Binary}
	}
	cmd := exec.Command(spec.Binary)
	cmd.Args = args
	cmd.Env = spec.Env
	cmd.Dir = spec.Dir
	cmd.Stdin = spec.Stdin
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = spec.Stdout
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = spec.Stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}

	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 1, err
}

func saveStateAsUser(acct *config.AccountInfo, lastSession string) error {
	if acct == nil {
		return nil
	}
	return SaveStateFn(acct.HomeDir, lastSession, acct.UID)
}
