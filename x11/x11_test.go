package x11

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type mockCharDeviceInfo struct {
	name string
	mode fs.FileMode
}

func (m mockCharDeviceInfo) Name() string       { return m.name }
func (m mockCharDeviceInfo) Size() int64        { return 0 }
func (m mockCharDeviceInfo) Mode() fs.FileMode  { return m.mode }
func (m mockCharDeviceInfo) ModTime() time.Time { return time.Time{} }
func (m mockCharDeviceInfo) IsDir() bool        { return false }
func (m mockCharDeviceInfo) Sys() any           { return nil }

type mockExecProxy struct {
	commandFn  func(name string, arg ...string) *exec.Cmd
	lookPathFn func(file string) (string, error)
}

func (m mockExecProxy) Command(name string, arg ...string) *exec.Cmd {
	return m.commandFn(name, arg...)
}

func (m mockExecProxy) LookPath(file string) (string, error) {
	if m.lookPathFn == nil {
		return "/mock/" + file, nil
	}
	return m.lookPathFn(file)
}

func helperCommand(name string, arg ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", name}
	cs = append(cs, arg...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestFindFreeDisplay(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := X11SocketDir
	X11SocketDir = tmpDir
	defer func() { X11SocketDir = origDir }()

	// Create a mock active display at 0 (by creating a directory to simulate socket and maybe making dialing fail appropriately or just testing IsDisplayActive behavior)
	// Actually, just making it an empty directory means no sockets exist, so all displays should be free
	display, err := FindFreeDisplay()
	if err != nil {
		t.Fatalf("FindFreeDisplay failed: %v", err)
	}
	if display != 0 {
		t.Errorf("FindFreeDisplay returned invalid display number: %d, expected 0", display)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd := args[0]
	if cmd == "chvt" {
		if os.Getenv("MOCK_CHVT_FAIL") == "1" {
			fmt.Fprintln(os.Stderr, "chvt failed")
			os.Exit(1)
		}
		os.Exit(0)
	}
	if cmd == "tty" {
		fmt.Println("/dev/tty1")
		os.Exit(0)
	}
	if cmd == "startx" {
		os.Exit(0)
	}
}

func TestGetVT(t *testing.T) {
	origExec := osExec
	origStat := osStat
	t.Cleanup(func() {
		osExec = origExec
		osStat = origStat
	})
	osExec = mockExecProxy{commandFn: helperCommand}
	osStat = func(name string) (os.FileInfo, error) {
		if name == "/dev/tty1" {
			return mockCharDeviceInfo{name: "tty1", mode: os.ModeCharDevice}, nil
		}
		return origStat(name)
	}

	vt, err := GetVT("keep", 0)
	if err != nil {
		t.Fatalf("GetVT keep failed: %v", err)
	}
	if vt != "1" {
		t.Errorf("Expected VT 1, got %s", vt)
	}

	vt, err = GetVT("7", 1)
	if err != nil {
		t.Fatalf("GetVT number failed: %v", err)
	}
	if vt != "8" {
		t.Errorf("Expected VT 8, got %s", vt)
	}

	_, err = GetVT("invalid", 1)
	if err == nil {
		t.Fatal("Expected error for invalid xtty format")
	}
}

func TestLaunchXSession(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}

	tmpLog, err := os.CreateTemp("", "startx_log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpLog.Name())

	err = LaunchXSession([]string{"/bin/sh"}, 1, "8", false, 30, false, tmpLog.Name(), []string{"-nolisten", "tcp"}, nil)
	if err != nil {
		t.Fatalf("LaunchXSession failed: %v", err)
	}
}

func TestLaunchXSessionSwitchVTFailure(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "MOCK_CHVT_FAIL=1")
		return cmd
	}}

	tmpLog, err := os.CreateTemp("", "startx_log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpLog.Name())

	err = LaunchXSession([]string{"/bin/sh"}, 1, "8", false, 30, false, tmpLog.Name(), []string{"-nolisten", "tcp"}, nil)
	if err == nil {
		t.Fatal("expected LaunchXSession to fail when VT handoff fails")
	}
}

func TestIsDisplayActive(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := X11SocketDir
	X11SocketDir = tmpDir
	defer func() { X11SocketDir = origDir }()

	// Since we mock X11SocketDir to an empty dir, no socket exists
	active := IsDisplayActive(0)
	if active {
		t.Errorf("Expected display 0 to be inactive when no socket exists")
	}
}

func TestSwitchVT(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}

	err := SwitchVT("7")
	if err != nil {
		t.Errorf("Expected SwitchVT to succeed, got err: %v", err)
	}
}
func TestLaunchXSessionMissingTool(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{
		commandFn: helperCommand,
		lookPathFn: func(file string) (string, error) {
			if file == "startx" {
				return "", fmt.Errorf("not found")
			}
			return "/mock/" + file, nil
		},
	}

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	err := LaunchXSession([]string{"/bin/sh"}, 1, "8", false, 30, false, logPath, nil, nil)
	if err == nil {
		t.Fatal("expected missing startx error")
	}
}

func TestLaunchXSessionAltStartXDeprecated(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}

	logPath := filepath.Join(t.TempDir(), "test.log")
	err := LaunchXSession([]string{"/bin/sh"}, 1, "8", false, 30, true, logPath, nil, nil)
	if err == nil {
		t.Fatal("expected altStartX deprecation error")
	}
}

func TestLaunchXSessionCKTimeoutDeprecated(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}

	logPath := filepath.Join(t.TempDir(), "test.log")
	err := LaunchXSession([]string{"/bin/sh"}, 1, "8", true, 60, false, logPath, nil, nil)
	if err == nil {
		t.Fatal("expected ckTimeout deprecation error")
	}
}

func TestLaunchXSessionInvalidLogPath(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}
	invalidPaths := []string{
		"../../tmp/test.log",
		"/var/log/../../../etc/shadow",
		"../",
		"../relative/path/../file.log",
	}

	for _, p := range invalidPaths {
		err := LaunchXSession([]string{"/bin/sh"}, 1, "8", false, 30, false, p, []string{"-nolisten", "tcp"}, nil)
		if err == nil {
			t.Fatalf("expected LaunchXSession to fail due to path traversal in startXLog for path %q", p)
		}
		if !strings.Contains(err.Error(), "path traversal is not allowed") {
			t.Fatalf("expected path traversal error for %q, got: %v", p, err)
		}
	}

	tmpDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// Verify valid paths pass validation inside an isolated temp working directory.
	validPaths := []string{
		"/dev/null",
		filepath.Join(tmpDir, "test.log"),
		"my..file.log",
		"valid.log",
	}

	for _, p := range validPaths {
		err := LaunchXSession([]string{"/bin/sh"}, 1, "8", false, 30, false, p, []string{"-nolisten", "tcp"}, nil)
		if err != nil && strings.Contains(err.Error(), "path traversal is not allowed") {
			t.Fatalf("expected LaunchXSession to NOT fail with path traversal error for valid path %q, got: %v", p, err)
		}
	}
}

func TestLaunchXSessionEmptyBin(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}
	err := LaunchXSession([]string{}, 1, "8", false, 30, false, filepath.Join(t.TempDir(), "test.log"), nil, nil)
	if err == nil {
		t.Fatal("expected error when session command is empty")
	}
	if err.Error() != "session command cannot be empty" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLaunchXSessionIllegalArguments(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}

	err := LaunchXSession([]string{"my-wm", "--", "malicious-arg"}, 1, "8", false, 30, false, filepath.Join(t.TempDir(), "test.log"), nil, nil)
	if err == nil {
		t.Fatal("expected error when session command contains '--'")
	}
	if err.Error() != "session command cannot contain '--'" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLaunchXSessionUnresolvedBinary(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{
		commandFn: helperCommand,
		lookPathFn: func(file string) (string, error) {
			if file == "nonexistent-wm" {
				return "", fmt.Errorf("executable file not found in $PATH")
			}
			return "/mock/" + file, nil
		},
	}

	err := LaunchXSession([]string{"nonexistent-wm"}, 1, "8", false, 30, false, filepath.Join(t.TempDir(), "test.log"), nil, nil)
	if err == nil {
		t.Fatal("expected error when session binary cannot be resolved")
	}
}

func TestLaunchXSessionWithOptionsSkipsChvtWhenAlreadyOnVT(t *testing.T) {
	origExec := osExec
	t.Cleanup(func() { osExec = origExec })

	osExec = mockExecProxy{
		lookPathFn: func(file string) (string, error) {
			if file == "chvt" {
				return "", fmt.Errorf("chvt missing")
			}
			return "/mock/" + file, nil
		},
		commandFn: func(name string, arg ...string) *exec.Cmd {
			if name == "chvt" {
				t.Fatalf("did not expect chvt to run when CurrentVT == VT (%v)", arg)
			}
			return helperCommand(name, arg...)
		},
	}

	logPath := filepath.Join(t.TempDir(), "startx.log")
	err := LaunchXSessionWithOptions(LaunchOptions{
		Bin:       []string{"/bin/sh"},
		Display:   0,
		VT:        "1",
		CurrentVT: "1",
		StartXLog: logPath,
	})
	if err != nil {
		t.Fatalf("expected CurrentVT == VT to skip chvt cleanly, got: %v", err)
	}
}

func TestLaunchXSessionWithOptionsRejectsSymlinkLog(t *testing.T) {
	origExec := osExec
	t.Cleanup(func() { osExec = origExec })
	osExec = mockExecProxy{commandFn: helperCommand}

	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "real.log")
	if err := os.WriteFile(realFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	symPath := filepath.Join(tmpDir, "sym.log")
	if err := os.Symlink(realFile, symPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	err := LaunchXSessionWithOptions(LaunchOptions{
		Bin:       []string{"/bin/sh"},
		Display:   0,
		VT:        "2",
		CurrentVT: "2",
		StartXLog: symPath,
	})
	if err == nil || !strings.Contains(err.Error(), "symlinks are not allowed") {
		t.Fatalf("expected symlink startXLog error, got %v", err)
	}
}
