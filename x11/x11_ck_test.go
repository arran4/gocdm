package x11

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLaunchXSessionConsoleKit(t *testing.T) {
	origExec := osExec
	defer func() { osExec = origExec }()

	var capturedArgs []string
	osExec = mockExecProxy{commandFn: func(name string, arg ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, arg...)
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}}

	tmpLog, err := os.CreateTemp("", "startx_log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpLog.Name())

	err = LaunchXSession([]string{"my-wm"}, 1, "8", true, 30, false, tmpLog.Name(), []string{"-nolisten", "tcp"}, []string{"USER=demo"})
	if err != nil {
		t.Fatalf("LaunchXSession failed: %v", err)
	}

	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "ck-launch-session /mock/my-wm") {
		t.Errorf("Expected startx arguments to contain 'ck-launch-session /mock/my-wm', got: %v", capturedArgs)
	}
}
