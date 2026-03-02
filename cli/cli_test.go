package cli

import (
	"fmt"
	"github.com/arran4/gocdm/x11"
	"os"
	"os/exec"
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
	Run([]string{"-help"}, mockExit)

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

	Run([]string{"-version"}, mockExit)

	if !exited {
		t.Error("Expected exit(0) for -version, but did not exit")
	}
	if code != 0 {
		t.Errorf("Expected exit code 0 for -version, got %d", code)
	}
}

func TestValidateTTYNonDryRunRequiresTerminal(t *testing.T) {
	original := IsTerminal
	t.Cleanup(func() { IsTerminal = original })
	IsTerminal = func(fd int) bool { return false }

	err := validateTTY(false)
	if err == nil {
		t.Fatal("expected validateTTY to fail when no TTY is attached")
	}
}

func TestValidateTTYDryRunSkipsTerminalRequirement(t *testing.T) {
	original := IsTerminal
	t.Cleanup(func() { IsTerminal = original })
	IsTerminal = func(fd int) bool { return false }

	if err := validateTTY(true); err != nil {
		t.Fatalf("expected dry-run TTY validation to pass, got %v", err)
	}
}

func mockEnvFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "env-mocks")
	if err != nil {
		t.Fatal(err)
	}

	origPasswdFilePath := PasswdFilePath
	origPamEnvConfPath := PamEnvConfPath
	PasswdFilePath = tmpDir + "/passwd"
	PamEnvConfPath = tmpDir + "/pam_env.conf"

	if err := os.WriteFile(PasswdFilePath, []byte(currentUsername()+":x:1000:1000:User:/home/user:/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PamEnvConfPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		PasswdFilePath = origPasswdFilePath
		PamEnvConfPath = origPamEnvConfPath
		os.RemoveAll(tmpDir)
	})
}

func TestRunDryRunNoSessions(t *testing.T) {
	mockEnvFiles(t)
	// Create temp home with no sessions
	tmpHome, err := os.MkdirTemp("", "test_home")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpHome); err != nil {
			t.Errorf("cleanup failed: %v", err)
		}
	})

	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CONFIG_HOME", tmpHome)

	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	Run([]string{"-dry-run"}, mockExit)

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

	Run([]string{"-unknown-flag"}, mockExit)

	if !exited {
		t.Error("Expected exit(2) for unknown flag, but did not exit")
	}
	if code != 2 {
		t.Errorf("Expected exit code 2 for unknown flag, got %d", code)
	}
}

func TestRunWaylandSessionDryRun(t *testing.T) {
	mockEnvFiles(t)
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("cleanup failed: %v", err)
		}
	})

	configPath := tmpDir + "/cdmrc"
	content := `binlist=("sway")
namelist=("Sway")
flaglist=("W")`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

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

	// Should do dry run and return successfully
	Run([]string{"-config", configPath, "-dry-run"}, mockExit)

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = oldStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if exited {
		t.Errorf("Expected dry run to return without exit, but exited with %d", code)
	}

	expectedOutput := "Dry run: would execute Wayland program: sway []\n"
	if output != expectedOutput {
		t.Errorf("Expected output %q, got %q", expectedOutput, output)
	}
}

func TestRunConsoleSessionDryRun(t *testing.T) {
	mockEnvFiles(t)
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("cleanup failed: %v", err)
		}
	})

	configPath := tmpDir + "/cdmrc"
	content := `binlist=("echo test")
namelist=("Test")
flaglist=("C")`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	// Should do dry run and return successfully
	Run([]string{"-config", configPath, "-dry-run"}, mockExit)

	if exited {
		t.Errorf("Expected dry run to return without exit, but exited with %d", code)
	}
}

func TestRunXSessionDryRun(t *testing.T) {
	mockEnvFiles(t)
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("cleanup failed: %v", err)
		}
	})

	configPath := tmpDir + "/cdmrc"
	content := `binlist=("startx")
namelist=("TestX")
flaglist=("X")`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	// Should do dry run and return successfully
	Run([]string{"-config", configPath, "-dry-run"}, mockExit)

	if exited {
		t.Errorf("Expected dry run to return without exit, but exited with %d", code)
	}
}

func TestRunConsoleEmptyCommand(t *testing.T) {
	mockEnvFiles(t)
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("cleanup failed: %v", err)
		}
	})

	configPath := tmpDir + "/cdmrc"
	content := `binlist=("")
namelist=("Empty")
flaglist=("C")`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	Run([]string{"-config", configPath, "-dry-run"}, mockExit)

	if !exited || code != 1 {
		t.Errorf("Expected exit(1) for empty command, got exited=%v code=%d", exited, code)
	}
}

func TestRunInvalidConfig(t *testing.T) {
	mockEnvFiles(t)
	exited := false
	code := -1
	mockExit := func(c int) {
		exited = true
		code = c
	}

	Run([]string{"-config", "/nonexistent/path/that/will/fail"}, mockExit)

	if !exited || code != 1 {
		t.Errorf("Expected exit(1) for invalid config, got exited=%v code=%d", exited, code)
	}
}

func TestRunXSessionLockTTYActive(t *testing.T) {
	mockEnvFiles(t)
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("cleanup failed: %v", err)
		}
	})

	origExecCommand := x11.ExecCommand
	defer func() { x11.ExecCommand = origExecCommand }()

	x11.ExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	configPath := tmpDir + "/cdmrc"
	content := `binlist=("startx")
namelist=("TestX")
flaglist=("X")
locktty=yes
display=0
xtty=keep`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

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
	Run([]string{"-config", configPath, "-dry-run"}, mockExit)

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
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
		// Simulates xdpyinfo success (active display)
		os.Exit(0)
	}
	if cmd == "tty" {
		// Output what the test expects: /dev/tty7
		fmt.Println("/dev/tty7")
		os.Exit(0)
	}
	if cmd == "chvt" {
		os.Exit(0)
	}
	// default
	os.Exit(1)
}
