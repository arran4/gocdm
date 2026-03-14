package x11

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type execProxy interface {
	Command(name string, arg ...string) *exec.Cmd
	LookPath(file string) (string, error)
}

type realExecProxy struct{}

var ExecCommand = exec.Command

func (realExecProxy) Command(name string, arg ...string) *exec.Cmd {
	return ExecCommand(name, arg...)
}

func (realExecProxy) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

var osExec execProxy = realExecProxy{}

func ensureTool(tool string) error {
	if _, err := osExec.LookPath(tool); err != nil {
		return fmt.Errorf("required tool %q not found in PATH", tool)
	}
	return nil
}

// IsDisplayActive checks if the given display number is active.
func IsDisplayActive(display int) bool {
	cmd := osExec.Command("xdpyinfo", "-display", fmt.Sprintf(":%d.0", display))
	output, _ := cmd.CombinedOutput()

	// If the command succeeds, the display is active.
	if cmd.ProcessState != nil && cmd.ProcessState.Success() {
		return true
	}

	outStr := string(output)
	if strings.Contains(outStr, "No protocol specified") || strings.Contains(outStr, "Invalid MIT") {
		// Display is active but inaccessible
		return true
	}

	return false
}

// SwitchVT switches the virtual terminal to the given VT number.
func SwitchVT(vt string) error {
	cmd := osExec.Command("chvt", vt)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to switch to VT %s: %w", vt, err)
	}
	return nil
}

// FindFreeDisplay finds the first available X display number starting from 0.
func FindFreeDisplay() (int, error) {
	if err := ensureTool("xdpyinfo"); err != nil {
		return -1, fmt.Errorf("cannot probe X displays: %w", err)
	}

	for i := 0; i < 7; i++ {
		cmd := osExec.Command("xdpyinfo", "-display", fmt.Sprintf(":%d.0", i))
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
		cmd := osExec.Command("tty")
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
func LaunchXSession(bin []string, display int, vt string, consoleKit bool, ckTimeout int, altStartX bool, startXLog string, serverArgs []string, env []string) error {
	if altStartX {
		return fmt.Errorf("altStartX is deprecated and not supported; set altstartx=no")
	}
	if consoleKit && ckTimeout != 30 {
		return fmt.Errorf("ckTimeout is deprecated and not supported; set cktimeout=30")
	}
	if err := ensureTool("chvt"); err != nil {
		return err
	}
	if err := ensureTool("startx"); err != nil {
		return err
	}
	if consoleKit {
		if err := ensureTool("ck-launch-session"); err != nil {
			return err
		}
	}

	if err := SwitchVT(vt); err != nil {
		return fmt.Errorf("failed VT handoff before launching X session: %w", err)
	}

	xArgs := []string{fmt.Sprintf(":%d", display)}
	xArgs = append(xArgs, serverArgs...)
	xArgs = append(xArgs, "vt"+vt)

	cmdArgs := []string{}
	if consoleKit {
		cmdArgs = append(cmdArgs, "ck-launch-session")
	}
	cmdArgs = append(cmdArgs, bin...)
	cmdArgs = append(cmdArgs, "--")
	cmdArgs = append(cmdArgs, xArgs...)

	cmd := osExec.Command("startx", cmdArgs...)
	cmd.SysProcAttr = newSysProcAttr()
	if len(env) > 0 {
		cmd.Env = append([]string{}, env...)
	}

	for _, part := range strings.Split(filepath.ToSlash(startXLog), "/") {
		if part == ".." {
			return fmt.Errorf("invalid startXLog path: path traversal is not allowed")
		}
	}

	outfile, err := os.OpenFile(startXLog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", startXLog, err)
	}
	defer outfile.Close()
	cmd.Stdout = outfile
	cmd.Stderr = outfile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigs {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	err = cmd.Wait()
	signal.Stop(sigs)
	close(sigs)
	return err
}
