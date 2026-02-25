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
	if cmd == "tty" {
		fmt.Println("/dev/tty1")
		os.Exit(0)
	}
	if cmd == "startx" {
		// Mock startx success
		os.Exit(0)
	}
}
