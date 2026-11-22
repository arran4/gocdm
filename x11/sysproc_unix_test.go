//go:build !windows

package x11

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLaunchXSessionWithOptionsForwardsSignalToProcessGroup(t *testing.T) {
	origExec := osExec
	t.Cleanup(func() { osExec = origExec })

	tmpDir := t.TempDir()
	readyFile := filepath.Join(tmpDir, "startx.ready")
	logFile := filepath.Join(tmpDir, "startx.log")

	osExec = mockExecProxy{
		lookPathFn: func(file string) (string, error) {
			return "/mock/" + file, nil
		},
		commandFn: func(name string, arg ...string) *exec.Cmd {
			if name == "chvt" {
				return helperCommand("chvt")
			}
			// Spawn a real subshell with a background sleep child in its own session (Setsid: true)
			// so signaling -pgid terminates both parent and child cleanly.
			return exec.Command("/bin/sh", "-c", "touch \"$1\"; sleep 10", "sh", readyFile)
		},
	}

	sigCh := make(chan os.Signal, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- launchXSessionWithSignals(LaunchOptions{
			Bin:       []string{"/usr/bin/xterm"},
			Display:   0,
			VT:        "2",
			CurrentVT: "2",
			StartXLog: logFile,
		}, sigCh)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(readyFile); err == nil {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if _, err := os.Stat(readyFile); err != nil {
		t.Fatalf("fake startx child did not start in time: %v", err)
	}

	sigCh <- syscall.SIGTERM

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from signaled child, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for X session child process group termination on SIGTERM")
	}
}

func TestOpenStartXLogRejectsFIFO(t *testing.T) {
	tmpDir := t.TempDir()
	fifoPath := filepath.Join(tmpDir, "evil.fifo")
	if err := syscall.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatalf("mkfifo failed: %v", err)
	}

	origExec := osExec
	t.Cleanup(func() { osExec = origExec })
	osExec = mockExecProxy{commandFn: helperCommand}

	err := LaunchXSessionWithOptions(LaunchOptions{
		Bin:       []string{"/bin/sh"},
		Display:   0,
		VT:        "2",
		CurrentVT: "2",
		StartXLog: fifoPath,
	})
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected non-regular file rejection for FIFO, got %v", err)
	}
}
