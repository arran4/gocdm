package x11

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

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
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}

	os.Setenv("MOCK_DISPLAY_STATUS_0", "taken")
	os.Setenv("MOCK_DISPLAY_STATUS_1", "free")
	defer func() {
		os.Unsetenv("MOCK_DISPLAY_STATUS_0")
		os.Unsetenv("MOCK_DISPLAY_STATUS_1")
	}()

	display, err := FindFreeDisplay()
	if err != nil {
		t.Fatalf("FindFreeDisplay failed: %v", err)
	}
	if display != 1 {
		t.Errorf("Expected display 1, got %d", display)
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

	cmd, args := args[0], args[1:]
	if cmd == "xdpyinfo" {
		if len(args) < 2 {
			os.Exit(2)
		}
		display := args[1]
		if display == ":0.0" && os.Getenv("MOCK_DISPLAY_STATUS_0") == "taken" {
			os.Exit(0)
		}
		if display == ":1.0" && os.Getenv("MOCK_DISPLAY_STATUS_1") == "free" {
			fmt.Fprintf(os.Stderr, "xdpyinfo:  unable to open display \"%s\"\n", display)
			os.Exit(1)
		}
		os.Exit(0)
	}
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
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}

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
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}

	os.Setenv("MOCK_DISPLAY_STATUS_0", "taken")
	defer os.Unsetenv("MOCK_DISPLAY_STATUS_0")
	if !IsDisplayActive(0) {
		t.Errorf("Expected display 0 to be active")
	}

	os.Setenv("MOCK_DISPLAY_STATUS_1", "free")
	defer os.Unsetenv("MOCK_DISPLAY_STATUS_1")
	if IsDisplayActive(1) {
		t.Errorf("Expected display 1 to be inactive")
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

func TestFindFreeDisplayMissingTool(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{
		commandFn: helperCommand,
		lookPathFn: func(file string) (string, error) {
			return "", fmt.Errorf("not found")
		},
	}

	_, err := FindFreeDisplay()
	if err == nil {
		t.Fatal("expected missing tool error")
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

	err := LaunchXSession([]string{"/bin/sh"}, 1, "8", false, 30, false, "/tmp/test.log", nil, nil)
	if err == nil {
		t.Fatal("expected missing startx error")
	}
}

func TestLaunchXSessionAltStartXDeprecated(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}

	err := LaunchXSession([]string{"/bin/sh"}, 1, "8", false, 30, true, "/tmp/test.log", nil, nil)
	if err == nil {
		t.Fatal("expected altStartX deprecation error")
	}
}

func TestLaunchXSessionCKTimeoutDeprecated(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()
	osExec = mockExecProxy{commandFn: helperCommand}

	err := LaunchXSession([]string{"/bin/sh"}, 1, "8", true, 60, false, "/tmp/test.log", nil, nil)
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

	// Verify valid paths pass validation. To do this, we need to mock out other LaunchXSession checks
	// like vt handoff, but for just validating the log path, we can rely on standard setup.
	validPaths := []string{
		"/dev/null",
		"/tmp/test.log",
		"my..file.log",
		"valid.log",
	}

	for _, p := range validPaths {
		// Mock os.OpenFile failure or success doesn't matter, we just care that it DOES NOT return the "path traversal" error
		err := LaunchXSession([]string{"/bin/sh"}, 1, "8", false, 30, false, p, []string{"-nolisten", "tcp"}, nil)
		if err != nil && strings.Contains(err.Error(), "path traversal is not allowed") {
			t.Fatalf("expected LaunchXSession to NOT fail with path traversal error for valid path %q, got: %v", p, err)
		}
	}
}
