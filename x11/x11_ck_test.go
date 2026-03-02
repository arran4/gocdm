package x11

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLaunchXSessionConsoleKit(t *testing.T) {
	origExecCommand := ExecCommand
	defer func() { ExecCommand = origExecCommand }()

	var capturedArgs []string
	ExecCommand = func(name string, arg ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, arg...)
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

	// Test with consoleKit=true
	err = LaunchXSession([]string{"my-wm"}, 1, "8", true, 30, false, tmpLog.Name(), []string{"-nolisten", "tcp"}, []string{"USER=demo"})
	if err != nil {
		t.Fatalf("LaunchXSession failed: %v", err)
	}

	// It should prepend ck-launch-session to the client binary arguments
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "ck-launch-session my-wm") {
		t.Errorf("Expected startx arguments to contain 'ck-launch-session my-wm', got: %v", capturedArgs)
	}
}
