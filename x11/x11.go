package x11

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

var execCommand = exec.Command

// FindFreeDisplay finds the first available X display number starting from 0.
func FindFreeDisplay() (int, error) {
	for i := 0; i < 7; i++ {
		cmd := execCommand("xdpyinfo", "-display", fmt.Sprintf(":%d.0", i))
		output, _ := cmd.CombinedOutput()

		// If command succeeded, display is active
		if cmd.ProcessState != nil && cmd.ProcessState.Success() {
			continue
		}

		outStr := string(output)
		if strings.Contains(outStr, "No protocol specified") || strings.Contains(outStr, "Invalid MIT") {
			// Display is in use but inaccessible
			continue
		}

		// Display is free
		return i, nil
	}
	return -1, fmt.Errorf("no free display found")
}

// GetVT returns the VT number to use.
// xtty is the configured XTTY setting (e.g. "7" or "keep").
// display is the X display number.
func GetVT(xtty string, display int) (string, error) {
	if xtty == "keep" {
		// Use current TTY
		cmd := execCommand("tty")
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		s := strings.TrimSpace(string(out))
		if strings.HasPrefix(s, "/dev/tty") {
			return strings.TrimPrefix(s, "/dev/tty"), nil
		}
		return "", fmt.Errorf("invalid tty: %s", s)
	}

	val, err := strconv.Atoi(xtty)
	if err != nil {
		return "", fmt.Errorf("invalid xtty value: %v", err)
	}
	return strconv.Itoa(val + display), nil
}

// LaunchXSession launches the X session using startx.
// bin: the command to execute (e.g., ["/usr/bin/xterm"]).
// display: the X display number.
// vt: the VT number.
// consoleKit: whether to use ConsoleKit monitoring.
// ckTimeout: timeout for ConsoleKit monitoring in seconds.
// altStartX: whether to use alternate startx launch method.
// startXLog: path to log file.
// serverArgs: additional arguments for X server.
func LaunchXSession(bin []string, display int, vt string, consoleKit bool, ckTimeout int, altStartX bool, startXLog string, serverArgs []string) error {
	// Construct X server arguments
	// e.g. :0 -nolisten tcp vt7
	xArgs := []string{fmt.Sprintf(":%d", display)}
	xArgs = append(xArgs, serverArgs...)
	xArgs = append(xArgs, "vt"+vt)

	// Construct startx arguments
	// startx client -- server_args
	cmdArgs := []string{}
	if consoleKit {
		cmdArgs = append(cmdArgs, "ck-launch-session")
	}
	cmdArgs = append(cmdArgs, bin...)
	cmdArgs = append(cmdArgs, "--")
	cmdArgs = append(cmdArgs, xArgs...)

	// Create command
	// We use Setsid to create a new session
	cmd := execCommand("startx", cmdArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Logging
	outfile, err := os.OpenFile(startXLog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", startXLog, err)
	}
	// We pass the file to the child process. It's safe to close in parent after Start.
	defer outfile.Close()
	cmd.Stdout = outfile
	cmd.Stderr = outfile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	// We detach and return. The process continues running.
	return nil
}
