package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/arran4/gocdm/auth"
	"github.com/arran4/gocdm/config"
	"github.com/arran4/gocdm/x11"
)

func TestSupervisorChildHelper(t *testing.T) {
	if os.Getenv("GO_WANT_SUPERVISOR_CHILD") != "1" {
		return
	}
	mode := os.Getenv("SUPERVISOR_CHILD_MODE")
	switch mode {
	case "probe":
		cwd, _ := os.Getwd()
		payload, _ := json.Marshal(map[string]string{
			"cwd": cwd,
			"val": os.Getenv("SUPERVISOR_PROBE_ENV"),
		})
		fmt.Println(string(payload))
		os.Exit(0)
	case "exit42":
		os.Exit(42)
	case "selfkill":
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		time.Sleep(5 * time.Second)
		os.Exit(0)
	case "sigint":
		_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
		time.Sleep(5 * time.Second)
		os.Exit(0)
	case "waitterm":
		readyFile := os.Getenv("SUPERVISOR_READY_FILE")
		if readyFile != "" {
			_ = os.WriteFile(readyFile, []byte("ready"), 0o644)
		}
		time.Sleep(10 * time.Second)
		os.Exit(0)
	default:
		os.Exit(99)
	}
}

func TestRunSupervisedSessionChildLifecycle(t *testing.T) {
	workDir := t.TempDir()
	resolvedWorkDir, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		t.Fatal(err)
	}

	makeSpec := func(mode string, extraEnv ...string) (SupervisedSessionSpec, *bytes.Buffer) {
		var stdout bytes.Buffer
		env := append([]string{
			"GO_WANT_SUPERVISOR_CHILD=1",
			"SUPERVISOR_CHILD_MODE=" + mode,
		}, extraEnv...)
		return SupervisedSessionSpec{
			Binary: os.Args[0],
			Args:   []string{os.Args[0], "-test.run=^TestSupervisorChildHelper$"},
			Env:    env,
			Dir:    resolvedWorkDir,
			Stdin:  strings.NewReader(""),
			Stdout: &stdout,
			Stderr: &bytes.Buffer{},
		}, &stdout
	}

	t.Run("exit 0 preserves cwd and env", func(t *testing.T) {
		spec, out := makeSpec("probe", "SUPERVISOR_PROBE_ENV=hello-pam")
		code, err := runSupervisedSession(spec)
		if err != nil || code != 0 {
			t.Fatalf("got (%d, %v), want (0, nil)", code, err)
		}
		var probe map[string]string
		if err := json.Unmarshal(out.Bytes(), &probe); err != nil {
			t.Fatalf("unmarshal probe %q: %v", out.String(), err)
		}
		if probe["cwd"] != resolvedWorkDir || probe["val"] != "hello-pam" {
			t.Fatalf("unexpected child probe: %+v", probe)
		}
	})

	t.Run("non-zero exit code propagated", func(t *testing.T) {
		spec, _ := makeSpec("exit42")
		code, err := runSupervisedSession(spec)
		if err != nil || code != 42 {
			t.Fatalf("got (%d, %v), want (42, nil)", code, err)
		}
	})

	t.Run("signaled child returns 128+signal", func(t *testing.T) {
		spec, _ := makeSpec("selfkill")
		code, err := runSupervisedSession(spec)
		if err != nil || code != 128+int(syscall.SIGTERM) {
			t.Fatalf("got (%d, %v), want (%d, nil)", code, err, 128+int(syscall.SIGTERM))
		}
	})

	t.Run("child keeps default SIGINT disposition (does not inherit SIG_IGN)", func(t *testing.T) {
		spec, _ := makeSpec("sigint")
		code, err := runSupervisedSession(spec)
		if err != nil || code != 128+int(syscall.SIGINT) {
			t.Fatalf("expected child SIGINT termination (%d), got (%d, %v)", 128+int(syscall.SIGINT), code, err)
		}
	})

	t.Run("SIGTERM forwarded to child process group", func(t *testing.T) {
		readyFile := filepath.Join(workDir, "child.ready")
		spec, _ := makeSpec("waitterm", "SUPERVISOR_READY_FILE="+readyFile)
		sigCh := make(chan os.Signal, 1)

		done := make(chan struct {
			code int
			err  error
		}, 1)
		go func() {
			c, e := runSupervisedSessionWithSignals(spec, sigCh)
			done <- struct {
				code int
				err  error
			}{c, e}
		}()

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, statErr := os.Stat(readyFile); statErr == nil {
				break
			}
			time.Sleep(15 * time.Millisecond)
		}
		if _, statErr := os.Stat(readyFile); statErr != nil {
			t.Fatalf("child did not become ready in time: %v", statErr)
		}

		sigCh <- syscall.SIGTERM

		select {
		case res := <-done:
			if res.err != nil || res.code != 128+int(syscall.SIGTERM) {
				t.Fatalf("got (%d, %v), want (%d, nil)", res.code, res.err, 128+int(syscall.SIGTERM))
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for child termination after forwarded SIGTERM")
		}
	})
}

func TestCliRunPAMNoticesRepeatedLoginsAndExitCleanup(t *testing.T) {
	setupGettyContextTestEnv(t)

	tmpDir := t.TempDir()
	aliceHome := filepath.Join(tmpDir, "alice")
	if err := os.MkdirAll(aliceHome, 0o755); err != nil {
		t.Fatal(err)
	}
	passwdPath := filepath.Join(tmpDir, "passwd")
	uid := os.Geteuid()
	if err := os.WriteFile(passwdPath, []byte(fmt.Sprintf("alice:x:%d:%d:Alice:%s:/bin/zsh\n", uid, uid, aliceHome)), 0o644); err != nil {
		t.Fatal(err)
	}
	pamEnvPath := filepath.Join(tmpDir, "pam_env.conf")
	if err := os.WriteFile(pamEnvPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	PasswdFilePath = passwdPath
	PamEnvConfPath = pamEnvPath

	cfgPath := filepath.Join(tmpDir, "cdmrc")
	if err := os.WriteFile(cfgPath, []byte("binlist=(\"echo ok\")\nnamelist=(\"Zsh\")\nflaglist=(\"C\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	osReadlink = func(p string) (string, error) {
		if p == "/proc/self/fd/0" {
			return "/dev/tty2", nil
		}
		return "", os.ErrNotExist
	}
	origStat := osStat
	osStat = func(p string) (os.FileInfo, error) {
		if p == "/dev/tty2" {
			return mockFileInfo{mode: os.ModeCharDevice}, nil
		}
		return origStat(p)
	}
	t.Cleanup(func() { osStat = origStat })

	TuiPromptCredentials = func(title, theme, version string, authFunc func(string, string) error) (string, string, error) {
		return "alice", "pw", authFunc("alice", "pw")
	}
	ExecLookPath = func(file string) (string, error) { return "/bin/" + file, nil }
	SaveStateAsUserFn = func(acct *config.AccountInfo, lastSession string) error {
		return config.SafeSaveStateAt(acct.HomeDir, lastSession, acct.UID)
	}

	t.Run("delivers notices once sets PAM VT metadata and cleans up on non-zero child exit", func(t *testing.T) {
		var events []string
		var noticeOut, noticeErr bytes.Buffer
		NoticesStdout = &noticeOut
		NoticesStderr = &noticeErr

		sess := &spyLoginSession{
			username: "alice",
			events:   &events,
			pamEnv:   []string{"XDG_RUNTIME_DIR=/run/user/1001"},
			notices: []auth.Notice{
				{Style: auth.NoticeInfo, Text: "Last login: Thu Nov 19 on tty2"},
				{Style: auth.NoticeInfo, Text: "   ASCII MOTD"},
				{Style: auth.NoticeError, Text: "Warning: quota near limit"},
			},
		}
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{events: &events, session: sess}
		}
		PrepareTTYFn = func(acct *config.AccountInfo) (TTYCleanup, string, error) {
			events = append(events, "prepare_tty")
			return func() error {
				events = append(events, "restore_tty")
				return nil
			}, "2", nil
		}
		RunSupervisedSessionFn = func(spec SupervisedSessionSpec) (int, error) {
			events = append(events, "supervise")
			return 17, nil
		}

		var exited bool
		var code int
		Run([]string{"-login", "-config", cfgPath}, func(c int) { exited = true; code = c })

		if !exited || code != 17 {
			t.Fatalf("expected child exit code 17, got exited=%v code=%d", exited, code)
		}
		if sess.pamPutEnv["XDG_SESSION_TYPE"] != "tty" || sess.pamPutEnv["XDG_VTNR"] != "2" || sess.pamPutEnv["XDG_SEAT"] != "seat0" {
			t.Fatalf("unexpected PAM metadata passed before OpenSession: %+v", sess.pamPutEnv)
		}
		if !strings.Contains(noticeOut.String(), "Last login: Thu Nov 19 on tty2\n   ASCII MOTD\n") {
			t.Fatalf("unexpected stdout notices: %q", noticeOut.String())
		}
		if noticeErr.String() != "Warning: quota near limit\n" {
			t.Fatalf("unexpected stderr notices: %q", noticeErr.String())
		}
		want := []string{
			"auth",
			"set_env:XDG_SESSION_TYPE",
			"set_env:XDG_SESSION_CLASS",
			"set_env:XDG_VTNR",
			"set_env:XDG_SEAT",
			"open_session",
			"drain_notices",
			"prepare_tty",
			"supervise",
			"restore_tty",
			"close_session",
		}
		if !reflect.DeepEqual(events, want) {
			t.Fatalf("lifecycle sequence = %v, want %v", events, want)
		}
	})

	t.Run("repeated sequential logins each open and close cleanly", func(t *testing.T) {
		for round := 1; round <= 2; round++ {
			var noticeOut, noticeErr bytes.Buffer
			NoticesStdout = &noticeOut
			NoticesStderr = &noticeErr
			var closed bool
			sess := &spyLoginSession{
				username:    "alice",
				closeCalled: &closed,
				notices:     []auth.Notice{{Style: auth.NoticeInfo, Text: fmt.Sprintf("MOTD-%d", round)}},
			}
			NewAuthenticator = func(string) auth.Authenticator {
				return spyAuthenticator{session: sess}
			}
			RunSupervisedSessionFn = func(spec SupervisedSessionSpec) (int, error) {
				return 0, nil
			}
			var exited bool
			var code int
			Run([]string{"-login", "-config", cfgPath}, func(c int) { exited = true; code = c })
			if !exited || code != 0 || !closed {
				t.Fatalf("round %d failed: exited=%v code=%d closed=%v", round, exited, code, closed)
			}
			if noticeOut.String() != fmt.Sprintf("MOTD-%d\n", round) {
				t.Fatalf("round %d noticeOut=%q", round, noticeOut.String())
			}
		}
	})

	t.Run("OpenSession failure skips child launch and exits 1", func(t *testing.T) {
		var closed bool
		sess := &spyLoginSession{
			username:    "alice",
			openErr:     errors.New("open failed"),
			closeCalled: &closed,
		}
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{session: sess}
		}
		RunSupervisedSessionFn = func(SupervisedSessionSpec) (int, error) {
			t.Fatal("must not start child when OpenSession fails")
			return 0, nil
		}
		var exited bool
		var code int
		Run([]string{"-login", "-config", cfgPath}, func(c int) { exited = true; code = c })
		if !exited || code != 1 || !closed {
			t.Fatalf("expected exit(1) and closed=true, got exited=%v code=%d closed=%v", exited, code, closed)
		}
	})

	t.Run("X session supervision passes user credentials and closes PAM on exit", func(t *testing.T) {
		xCfg := filepath.Join(tmpDir, "x-cdmrc")
		if err := os.WriteFile(xCfg, []byte("binlist=(\"xterm\")\nnamelist=(\"XTerm\")\nflaglist=(\"X\")\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		var closed bool
		var events []string
		sess := &spyLoginSession{
			username:    "alice",
			events:      &events,
			closeCalled: &closed,
		}
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{events: &events, session: sess}
		}
		PrepareTTYFn = func(acct *config.AccountInfo) (TTYCleanup, string, error) {
			events = append(events, "prepare_tty")
			return func() error {
				events = append(events, "restore_tty")
				return nil
			}, "2", nil
		}

		var launchOptsUsed bool
		LaunchSupervisedXSessionFn = func(opts x11.LaunchOptions) error {
			launchOptsUsed = true
			events = append(events, "launch_x")
			if opts.UID == nil || *opts.UID != uid || opts.Dir != aliceHome {
				return fmt.Errorf("unexpected launch opts: %+v", opts)
			}
			return nil
		}

		var exited bool
		var code int
		Run([]string{"-login", "-config", xCfg}, func(c int) { exited = true; code = c })
		if !exited || code != 0 || !closed || !launchOptsUsed {
			t.Fatalf("X supervision failed: exited=%v code=%d closed=%v launchOptsUsed=%v", exited, code, closed, launchOptsUsed)
		}
	})
}

func TestRunInternalSaveStateRefusesRoot(t *testing.T) {
	setupGettyContextTestEnv(t)
	osGeteuid = func() int { return 0 }
	t.Setenv("GOCDM_INTERNAL_SAVE_STATE", "1")
	t.Setenv("GOCDM_STATE_HOME", t.TempDir())
	t.Setenv("GOCDM_STATE_SESSION", "zsh")
	t.Setenv("GOCDM_STATE_UID", "1001")

	var exited bool
	var code int
	Run(nil, func(c int) { exited = true; code = c })
	if !exited || code != 1 {
		t.Fatalf("expected root internal state write to exit(1), got exited=%v code=%d", exited, code)
	}
}

func TestCompiledInternalSaveStateHelper(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("internal save state helper test expects non-root user")
	}
	bin := buildModeTestBinary(t)
	tmpHome := t.TempDir()

	cmd := exec.Command(bin)
	cmd.Env = []string{
		"GOCDM_INTERNAL_SAVE_STATE=1",
		"GOCDM_STATE_HOME=" + tmpHome,
		"GOCDM_STATE_SESSION=zsh",
		fmt.Sprintf("GOCDM_STATE_UID=%d", os.Geteuid()),
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("internal save-state helper failed: %v\n%s", err, string(out))
	}
	state, err := config.LoadStateForUser(tmpHome, os.Geteuid())
	if err != nil || state.LastSession != "zsh" {
		t.Fatalf("expected saved state 'zsh', got (%+v, %v)", state, err)
	}
}

func TestAccountCredentialFailClosedAsRoot(t *testing.T) {
	origKernelEuid := kernelGeteuid
	t.Cleanup(func() { kernelGeteuid = origKernelEuid })
	kernelGeteuid = func() int { return 0 }

	if _, err := accountCredential(nil); err == nil {
		t.Fatal("expected error when running as root with nil AccountInfo")
	}
	if _, err := accountCredential(&config.AccountInfo{Username: "bad", UID: -1, GID: 1000}); err == nil {
		t.Fatal("expected error when running as root with negative UID")
	}
	if _, err := accountCredential(&config.AccountInfo{Username: "bad", UID: 1000, GID: -1}); err == nil {
		t.Fatal("expected error when running as root with negative GID")
	}

	cred, err := accountCredential(&config.AccountInfo{
		Username: "alice",
		UID:      1001,
		GID:      1001,
		Groups:   []int{1001, 27, 44},
	})
	if err != nil {
		t.Fatalf("expected valid credential for non-negative root transition, got %v", err)
	}
	if cred == nil || cred.Uid != 1001 || cred.Gid != 1001 || !reflect.DeepEqual(cred.Groups, []uint32{1001, 27, 44}) {
		t.Fatalf("unexpected credential: %+v", cred)
	}
}
