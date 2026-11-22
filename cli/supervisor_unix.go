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
	"sync"
	"syscall"

	"github.com/arran4/gocdm/config"
	"golang.org/x/sys/unix"
)

var kernelGeteuid = os.Geteuid

func accountCredential(acct *config.AccountInfo) (*syscall.Credential, error) {
	if kernelGeteuid() != 0 {
		return nil, nil
	}
	if acct == nil || acct.UID < 0 || acct.GID < 0 {
		return nil, fmt.Errorf("refusing to launch session as root without a valid authenticated account")
	}
	groups := make([]uint32, 0, len(acct.Groups)+1)
	for _, g := range acct.Groups {
		if g >= 0 {
			groups = append(groups, uint32(g))
		}
	}
	if len(groups) == 0 {
		groups = append(groups, uint32(acct.GID))
	}
	return &syscall.Credential{
		Uid:    uint32(acct.UID),
		Gid:    uint32(acct.GID),
		Groups: groups,
	}, nil
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
	defer func() { _ = devTTY.Close() }()

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
	cred, err := accountCredential(spec.Account)
	if err != nil {
		return 1, err
	}
	if cred != nil {
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

	// Catch (rather than signal.Ignore) interactive terminal/job-control signals so Go's
	// execve resets child dispositions to SIG_DFL instead of leaking SIG_IGN to children.
	drainSigs := make(chan os.Signal, 8)
	signal.Notify(drainSigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTSTP, syscall.SIGTTIN, syscall.SIGTTOU)
	stopDrain := make(chan struct{})
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for {
			select {
			case <-stopDrain:
				return
			case _, ok := <-drainSigs:
				if !ok {
					return
				}
			}
		}
	}()
	var stopDrainOnce sync.Once
	cleanupDrain := func() {
		stopDrainOnce.Do(func() {
			signal.Stop(drainSigs)
			close(stopDrain)
			<-drainDone
		})
	}
	defer cleanupDrain()

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
	cleanupDrain()

	if haveForeground && origPgrp > 0 {
		signal.Ignore(syscall.SIGTTOU)
		_ = unix.IoctlSetPointerInt(ttyFD, unix.TIOCSPGRP, origPgrp)
		signal.Reset(syscall.SIGTTOU)
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
	if kernelGeteuid() != 0 || acct.UID == 0 {
		return SaveStateFn(acct.HomeDir, lastSession, acct.UID)
	}

	cred, err := accountCredential(acct)
	if err != nil {
		return err
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
		Credential: cred,
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
