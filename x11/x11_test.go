package x11

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestFindFreeDisplay(t *testing.T) {
	// Mock execCommand
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	// We simulate display 0 is taken (returns 0), display 1 is free (returns 1 and message).
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
		// args: -display :X.0
		if len(args) < 2 {
			os.Exit(2)
		}
		display := args[1] // :0.0
		if display == ":0.0" && os.Getenv("MOCK_DISPLAY_STATUS_0") == "taken" {
			os.Exit(0)
		}
		if display == ":1.0" && os.Getenv("MOCK_DISPLAY_STATUS_1") == "free" {
			fmt.Fprintf(os.Stderr, "xdpyinfo:  unable to open display \"%s\"\n", display)
			os.Exit(1)
		}
		// Default to taken (0) to allow loop to continue if not mocked specifically
		os.Exit(0)
	}
	if cmd == "chvt" {
		os.Exit(0)
	}
	if cmd == "tty" {
		fmt.Println("/dev/tty1")
		os.Exit(0)
	}
	if cmd == "startx" {
		// Mock startx success
		os.Exit(0)
	}
}

func TestGetVT(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	// Test "keep" with valid tty output
	vt, err := GetVT("keep", 0)
	if err != nil {
		t.Fatalf("GetVT keep failed: %v", err)
	}
	if vt != "1" {
		t.Errorf("Expected VT 1, got %s", vt)
	}

	// Test number format
	vt, err = GetVT("7", 1)
	if err != nil {
		t.Fatalf("GetVT number failed: %v", err)
	}
	if vt != "8" {
		t.Errorf("Expected VT 8, got %s", vt)
	}

	// Test invalid number format
	_, err = GetVT("invalid", 1)
	if err == nil {
		t.Fatal("Expected error for invalid xtty format")
	}
}

func TestLaunchXSession(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

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

func TestIsDisplayActive(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

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
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	err := SwitchVT("7")
	if err != nil {
		t.Errorf("Expected SwitchVT to succeed, got err: %v", err)
	}
}
