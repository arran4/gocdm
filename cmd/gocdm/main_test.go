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

func TestRunVersion(t *testing.T) {
    exited := false
    code := -1
    mockExit := func(c int) {
        exited = true
        code = c
    }

    run([]string{"-version"}, mockExit)

    if !exited {
        t.Error("Expected exit(0) for -version, but did not exit")
    }
    if code != 0 {
        t.Errorf("Expected exit code 0 for -version, got %d", code)
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

func TestRunConsoleSessionDryRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := tmpDir + "/cdmrc"
	content := `binlist=("echo test")
namelist=("Test")
flaglist=("C")`
	os.WriteFile(configPath, []byte(content), 0644)

	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	// Should do dry run and return successfully
	run([]string{"-config", configPath, "-dry-run"}, mockExit)

	if exited {
		t.Errorf("Expected dry run to return without exit, but exited with %d", code)
	}
}

func TestRunXSessionDryRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := tmpDir + "/cdmrc"
	content := `binlist=("startx")
namelist=("TestX")
flaglist=("X")`
	os.WriteFile(configPath, []byte(content), 0644)

	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	// Should do dry run and return successfully
	run([]string{"-config", configPath, "-dry-run"}, mockExit)

	if exited {
		t.Errorf("Expected dry run to return without exit, but exited with %d", code)
	}
}

func TestRunConsoleEmptyCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := tmpDir + "/cdmrc"
	content := `binlist=("")
namelist=("Empty")
flaglist=("C")`
	os.WriteFile(configPath, []byte(content), 0644)

	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	run([]string{"-config", configPath, "-dry-run"}, mockExit)

	if !exited || code != 1 {
		t.Errorf("Expected exit(1) for empty command, got exited=%v code=%d", exited, code)
	}
}

func TestRunInvalidConfig(t *testing.T) {
	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	run([]string{"-config", "/nonexistent/path/that/will/fail"}, mockExit)

	if !exited || code != 1 {
		t.Errorf("Expected exit(1) for invalid config, got exited=%v code=%d", exited, code)
	}
}

func TestRunXSessionLockTTYActive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock xdpyinfo that exits 0 to simulate active display
	xdpyinfoPath := tmpDir + "/xdpyinfo"
	os.WriteFile(xdpyinfoPath, []byte("#!/bin/sh\nexit 0\n"), 0755)

	// Create a mock tty for GetVT("keep") just in case
	ttyPath := tmpDir + "/tty"
	os.WriteFile(ttyPath, []byte("#!/bin/sh\necho /dev/tty7\n"), 0755)

	// Prepend tmpDir to PATH so our mocks are found
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	configPath := tmpDir + "/cdmrc"
	content := `binlist=("startx")
namelist=("TestX")
flaglist=("X")
locktty=yes
display=0
xtty=keep`
	os.WriteFile(configPath, []byte(content), 0644)

	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// This tests the dry run logic with locktty parsed and mock xdpyinfo success
	run([]string{"-config", configPath, "-dry-run"}, mockExit)

	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if exited {
		t.Errorf("Expected dry run to return without exit, but exited with %d", code)
	}

	expectedOutput := "Dry run: would switch to existing X session on display :0 VT7\n"
	if output != expectedOutput {
		t.Errorf("Expected output %q, got %q", expectedOutput, output)
	}
}
