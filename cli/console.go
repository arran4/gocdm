package cli

import (
	"fmt"
	"path/filepath"

	"github.com/arran4/gocdm/config"
	"github.com/arran4/gocdm/session"
)

// ShellsFilePath is the login shells registry consulted for manual console entries.
var ShellsFilePath = session.ShellsFile

// ConsoleLaunchPlan describes how a console ("C") session should be executed.
type ConsoleLaunchPlan struct {
	Binary       string
	Args         []string
	IsLoginShell bool
	ShellEnv     string
}

// BuildConsoleLaunchPlan parses a console session command and resolves whether it should
// be invoked as a POSIX login shell (leading '-' in argv[0] for bare shells in getty mode)
// or executed directly as an ordinary console program / non-login shell.
func BuildConsoleLaunchPlan(
	sess session.Session,
	mode OperatingMode,
	lookPath func(string) (string, error),
	shellsFile string,
) (*ConsoleLaunchPlan, error) {
	parts, err := config.SplitCommand(sess.Exec)
	if err != nil {
		return nil, fmt.Errorf("invalid command %q: %w", sess.Exec, err)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("Empty command")
	}

	rawBin := parts[0]
	binary, err := lookPath(rawBin)
	if err != nil {
		return nil, fmt.Errorf("Command not found: %s", rawBin)
	}

	isShell := len(parts) == 1 && (sess.IsShell || session.IsRegisteredShell(rawBin, binary, shellsFile))
	if isShell && mode == ModeGettyLogin {
		loginArg0 := "-" + filepath.Base(binary)
		shellEnv := rawBin
		if !filepath.IsAbs(shellEnv) {
			shellEnv = binary
		}
		return &ConsoleLaunchPlan{
			Binary:       binary,
			Args:         []string{loginArg0},
			IsLoginShell: true,
			ShellEnv:     shellEnv,
		}, nil
	}

	return &ConsoleLaunchPlan{
		Binary:       binary,
		Args:         append([]string{rawBin}, parts[1:]...),
		IsLoginShell: false,
	}, nil
}
