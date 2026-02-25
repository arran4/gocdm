package ui

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestShowMenu(t *testing.T) {
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

	idx, err := ShowMenu("Title", []string{"A", "B"}, 0, "")
	if err != nil {
		t.Fatalf("ShowMenu failed: %v", err)
	}
	if idx != 0 {
		t.Errorf("Expected index 0, got %d", idx)
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
	if cmd != "dialog" {
		fmt.Fprintf(os.Stderr, "Expected command dialog, got %s\n", cmd)
		os.Exit(2)
	}

	// Output simulated selection (index 0)
	fmt.Print("0")
}
