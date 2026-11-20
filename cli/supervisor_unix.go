//go:build !windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/arran4/gocdm/config"
	"golang.org/x/sys/unix"
)

func accountCredential(acct *config.AccountInfo) *syscall.Credential {
	if osGeteuid() != 0 || acct == nil {
		return nil
	}
	groups := make([]uint32, 0, len(acct.Groups)+1)
	for _, g := range acct.Groups {
		if g >= 0 {
			groups = append(groups, uint32(g))
		}
	}
	if len(groups) == 0 && acct.GID >= 0 {
		groups = append(groups, uint32(acct.GID))
	}
	return &syscall.Credential{
		Uid:    uint32(acct.UID),
		Gid:    uint32(acct.GID),
		Groups: groups,
	}
}

func controllingTTYFGPgrp(f *os.File) (int, int, bool) {
	if f == nil || !IsTerminal(int(f.Fd())) {
		return 0, 0, false
	}
	fd := int(f.Fd())
	fdPgrp, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	if err != nil || fdPgrp <= 0 {
		return 0, 0, false
	}
	devTTY, err := os.Open("/dev/tty")
	if err != nil {
		return 0, 0, false
	}
	defer devTTY.Close()

	ctrlPgrp, err := unix.IoctlGetInt(int(devTTY.Fd()), unix.TIOCGPGRP)
	if err != nil || ctrlPgrp != fdPgrp {
		return 0, 0, false
	}
	return fd, fdPgrp, true
}

func runSupervisedSession(spec SupervisedSessionSpec) (int, error) {
	return runSupervisedSessionWithSignals(spec, nil)
}

func runSupervisedSessionWithSignals(spec SupervisedSessionSpec, testSigs <-chan os.Signal) (int, error) {
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

	sysAttr := &syscall.SysProcAttr{Setpgid: true}
	if cred := accountCredential(spec.Account); cred != nil {
		sysAttr.Credential = cred
	}

	var (
		ttyFD          int
		origPgrp       int
		haveForeground bool
	)
	if f, ok := cmd.Stdin.(*os.File); ok {
		ttyFD, origPgrp, haveForeground = controllingTTYFGPgrp(f)
		if haveForeground {
			sysAttr.Foreground = true
			sysAttr.Ctty = ttyFD
		}
	}
	cmd.SysProcAttr = sysAttr

	ignoredSigs := []os.Signal{syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTSTP, syscall.SIGTTIN, syscall.SIGTTOU}
	signal.Ignore(ignoredSigs...)
	defer signal.Reset(ignoredSigs...)

	if err := cmd.Start(); err != nil {
		return 1, err
	}

	var sigCh chan os.Signal
	var forwardSrc <-chan os.Signal
	if testSigs != nil {
		forwardSrc = testSigs
	} else {
		sigCh = make(chan os.Signal, 4)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)
		forwardSrc = sigCh
	}

	stopForward := make(chan struct{})
	forwardDone := make(chan struct{})
	childPID := cmd.Process.Pid
	go func() {
		defer close(forwardDone)
		for {
			select {
			case <-stopForward:
				return
			case sig, ok := <-forwardSrc:
				if !ok {
					return
				}
				if sysSig, ok := sig.(syscall.Signal); ok {
					_ = syscall.Kill(-childPID, sysSig)
				}
			}
		}
	}()

	waitErr := cmd.Wait()

	if sigCh != nil {
		signal.Stop(sigCh)
	}
	close(stopForward)
	<-forwardDone

	if haveForeground && origPgrp > 0 {
		_ = unix.IoctlSetPointerInt(ttyFD, unix.TIOCSPGRP, origPgrp)
	}

	if waitErr == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal()), nil
		}
		return exitErr.ExitCode(), nil
	}
	return 1, waitErr
}

func saveStateAsUser(acct *config.AccountInfo, lastSession string) error {
	if acct == nil {
		return fmt.Errorf("account info is required to save state")
	}
	if osGeteuid() != 0 || acct.UID == 0 {
		return SaveStateFn(acct.HomeDir, lastSession, acct.UID)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable for unprivileged state write: %w", err)
	}
	cmd := exec.Command(exePath)
	cmd.Env = []string{
		"GOCDM_INTERNAL_SAVE_STATE=1",
		"GOCDM_STATE_HOME=" + acct.HomeDir,
		"GOCDM_STATE_SESSION=" + lastSession,
		"GOCDM_STATE_UID=" + strconv.Itoa(acct.UID),
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: accountCredential(acct),
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%s (%w)", msg, err)
		}
		return err
	}
	return nil
}
