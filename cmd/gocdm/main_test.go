package main

import (
	"os"
	"testing"
)

func TestRunHelp(t *testing.T) {
	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	// We can't prevent flag from printing to stderr, but we can verify exit code.
	run([]string{"-help"}, mockExit)

	if !exited {
		t.Error("Expected exit(0) for -help, but did not exit")
	}
	if code != 0 {
		// Note: flag.PrintDefaults() is called but we return exit(0) manually in our code for ErrHelp
		t.Errorf("Expected exit code 0 for -help, got %d", code)
	}
}

func TestRunDryRunNoSessions(t *testing.T) {
	// Create temp home with no sessions
	tmpHome, err := os.MkdirTemp("", "test_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	oldHome := os.Getenv("HOME")
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("XDG_CONFIG_HOME", oldXDG)
	}()

	os.Setenv("HOME", tmpHome)
	os.Setenv("XDG_CONFIG_HOME", tmpHome)

	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	run([]string{"-dry-run"}, mockExit)

	// Since there are no sessions, it should exit(1) with "No sessions found."
	if !exited {
		t.Error("Expected exit(1) for no sessions, but did not exit")
	}
	if code != 1 {
		t.Errorf("Expected exit code 1 for no sessions, got %d", code)
	}
}

func TestRunUnknownFlag(t *testing.T) {
	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	// Silence stderr for this test as flag.Parse prints error
	// But flag.NewFlagSet uses os.Stderr by default.
	// We can't change it easily without returning the fs from a helper function, or modifying run signature.
	// Let's just accept the output or ignore it.

	run([]string{"-unknown-flag"}, mockExit)

	if !exited {
		t.Error("Expected exit(2) for unknown flag, but did not exit")
	}
	if code != 2 {
		t.Errorf("Expected exit code 2 for unknown flag, got %d", code)
	}
}
