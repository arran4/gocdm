package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/arran4/gocdm/auth"
	"github.com/arran4/gocdm/config"
	"github.com/arran4/gocdm/session"
)

type consoleProbeResult struct {
	Arg0   string   `json:"arg0"`
	Args   []string `json:"args"`
	CWD    string   `json:"cwd"`
	Shell  string   `json:"shell"`
	Home   string   `json:"home"`
	TrapOK bool     `json:"trap_ok"`
}

func TestConsoleArbitraryProbe(t *testing.T) {
	if os.Getenv("GO_WANT_CONSOLE_PROBE") != "1" {
		return
	}
	cwd, _ := os.Getwd()
	_, trapBash := os.Stat(filepath.Join(os.Getenv("HOME"), ".bash_trap_ran"))
	_, trapZsh := os.Stat(filepath.Join(os.Getenv("HOME"), ".zsh_trap_ran"))
	res := consoleProbeResult{
		Arg0:   os.Args[0],
		Args:   os.Args[1:],
		CWD:    cwd,
		Shell:  os.Getenv("SHELL"),
		Home:   os.Getenv("HOME"),
		TrapOK: os.IsNotExist(trapBash) && os.IsNotExist(trapZsh),
	}
	_ = json.NewEncoder(os.Stdout).Encode(res)
	os.Exit(0)
}

func TestBuildConsoleLaunchPlan(t *testing.T) {
	tmpDir := t.TempDir()
	shellsPath := filepath.Join(tmpDir, "shells")
	zshPath := filepath.Join(tmpDir, "zsh")
	bashPath := filepath.Join(tmpDir, "bash")
	htopPath := filepath.Join(tmpDir, "htop")
	for _, p := range []string{zshPath, bashPath, htopPath} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(shellsPath, []byte(zshPath+"\n"+bashPath+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	lookPath := func(name string) (string, error) {
		switch name {
		case zshPath, "zsh":
			return zshPath, nil
		case bashPath, "bash":
			return bashPath, nil
		case htopPath, "htop":
			return htopPath, nil
		default:
			return "", os.ErrNotExist
		}
	}

	t.Run("discovered shell in getty mode uses leading dash argv[0]", func(t *testing.T) {
		plan, err := BuildConsoleLaunchPlan(
			session.Session{Name: "zsh", Exec: zshPath, Type: "C", IsShell: true},
			ModeGettyLogin,
			lookPath,
			shellsPath,
		)
		if err != nil {
			t.Fatalf("BuildConsoleLaunchPlan failed: %v", err)
		}
		if !plan.IsLoginShell || plan.Binary != zshPath || !reflect.DeepEqual(plan.Args, []string{"-zsh"}) || plan.ShellEnv != zshPath {
			t.Fatalf("unexpected plan: %+v", plan)
		}
	})

	t.Run("discovered shell in user autostart mode does not use login argv[0]", func(t *testing.T) {
		plan, err := BuildConsoleLaunchPlan(
			session.Session{Name: "zsh", Exec: zshPath, Type: "C", IsShell: true},
			ModeUserSession,
			lookPath,
			shellsPath,
		)
		if err != nil {
			t.Fatalf("BuildConsoleLaunchPlan failed: %v", err)
		}
		if plan.IsLoginShell || plan.Binary != zshPath || !reflect.DeepEqual(plan.Args, []string{zshPath}) || plan.ShellEnv != "" {
			t.Fatalf("unexpected user-mode plan: %+v", plan)
		}
	})

	t.Run("manual cdmrc bare shell matches shells file in getty mode", func(t *testing.T) {
		plan, err := BuildConsoleLaunchPlan(
			session.Session{Name: "Bash", Exec: bashPath, Type: "C"},
			ModeGettyLogin,
			lookPath,
			shellsPath,
		)
		if err != nil {
			t.Fatalf("BuildConsoleLaunchPlan failed: %v", err)
		}
		if !plan.IsLoginShell || !reflect.DeepEqual(plan.Args, []string{"-bash"}) || plan.ShellEnv != bashPath {
			t.Fatalf("unexpected manual shell plan: %+v", plan)
		}
	})

	t.Run("shell with explicit args is not turned into login shell and preserves quotes", func(t *testing.T) {
		plan, err := BuildConsoleLaunchPlan(
			session.Session{Name: "Custom", Exec: bashPath + " -c 'echo hello world'", Type: "C"},
			ModeGettyLogin,
			lookPath,
			shellsPath,
		)
		if err != nil {
			t.Fatalf("BuildConsoleLaunchPlan failed: %v", err)
		}
		wantArgs := []string{bashPath, "-c", "echo hello world"}
		if plan.IsLoginShell || !reflect.DeepEqual(plan.Args, wantArgs) || plan.ShellEnv != "" {
			t.Fatalf("unexpected shell-with-args plan: %+v", plan)
		}
	})

	t.Run("arbitrary console executable runs directly", func(t *testing.T) {
		plan, err := BuildConsoleLaunchPlan(
			session.Session{Name: "Htop", Exec: "htop -d 10", Type: "C"},
			ModeGettyLogin,
			lookPath,
			shellsPath,
		)
		if err != nil {
			t.Fatalf("BuildConsoleLaunchPlan failed: %v", err)
		}
		if plan.IsLoginShell || plan.Binary != htopPath || !reflect.DeepEqual(plan.Args, []string{"htop", "-d", "10"}) || plan.ShellEnv != "" {
			t.Fatalf("unexpected arbitrary command plan: %+v", plan)
		}
	})
}

func TestRealBashLoginAndUserModes(t *testing.T) {
	bashBin, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}

	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedHome, err := filepath.EvalSymlinks(homeDir)
	if err != nil {
		t.Fatal(err)
	}

	shellsFile := filepath.Join(tmpDir, "shells")
	if err := os.WriteFile(shellsFile, []byte(bashBin+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	acct := &config.AccountInfo{
		Username: "tester",
		UID:      os.Geteuid(),
		GID:      os.Getegid(),
		HomeDir:  resolvedHome,
		Shell:    "/bin/sh",
	}

	t.Run("getty mode launches real bash as login shell in user home", func(t *testing.T) {
		logFile := filepath.Join(tmpDir, "bash_login.log")
		profile := fmt.Sprintf(
			"shopt -q login_shell && LOGIN=yes || LOGIN=no\n"+
				"printf 'PROFILE=yes\\nLOGIN=%%s\\nARG0=%%s\\nPWD=%%s\\nHOME=%%s\\nSHELL=%%s\\nSPAWN=%%s\\n' \"$LOGIN\" \"$0\" \"$PWD\" \"$HOME\" \"$SHELL\" \"$GOCDM_SPAWN\" > %q\n",
			logFile,
		)
		if err := os.WriteFile(filepath.Join(resolvedHome, ".bash_profile"), []byte(profile), 0o644); err != nil {
			t.Fatal(err)
		}

		plan, err := BuildConsoleLaunchPlan(
			session.Session{Name: "bash", Exec: bashBin, Type: "C", IsShell: true},
			ModeGettyLogin,
			exec.LookPath,
			shellsFile,
		)
		if err != nil {
			t.Fatalf("BuildConsoleLaunchPlan failed: %v", err)
		}

		env, err := config.BuildGettySessionEnv(config.GettyEnvOptions{
			Account:     acct,
			SessionType: "C",
			SessionName: "bash",
			WorkingDir:  resolvedHome,
			SpawnPID:    os.Getpid(),
		})
		if err != nil {
			t.Fatalf("BuildGettySessionEnv failed: %v", err)
		}
		if plan.ShellEnv != "" {
			env = config.SetEnvValue(env, "SHELL", plan.ShellEnv)
		}

		var stdout, stderr bytes.Buffer
		code, err := runSupervisedSession(SupervisedSessionSpec{
			Binary: plan.Binary,
			Args:   plan.Args,
			Env:    env,
			Dir:    resolvedHome,
			Stdin:  strings.NewReader("exit 23\n"),
			Stdout: &stdout,
			Stderr: &stderr,
		})
		if err != nil {
			t.Fatalf("runSupervisedSession failed: %v (stderr=%q)", err, stderr.String())
		}
		if code != 23 {
			t.Fatalf("exit code = %d, want 23", code)
		}

		data, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("expected .bash_profile to write %s: %v", logFile, err)
		}
		text := string(data)
		for _, want := range []string{
			"PROFILE=yes",
			"LOGIN=yes",
			"ARG0=-" + filepath.Base(bashBin),
			"PWD=" + resolvedHome,
			"HOME=" + resolvedHome,
			"SHELL=" + bashBin,
			fmt.Sprintf("SPAWN=%d", os.Getpid()),
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in bash login log, got:\n%s", want, text)
			}
		}
	})

	t.Run("user mode does not invoke login shell or duplicate .bash_profile", func(t *testing.T) {
		marker := filepath.Join(tmpDir, "user_mode_ran_profile")
		if err := os.WriteFile(filepath.Join(resolvedHome, ".bash_profile"), []byte("touch "+marker+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		plan, err := BuildConsoleLaunchPlan(
			session.Session{Name: "bash", Exec: bashBin, Type: "C", IsShell: true},
			ModeUserSession,
			exec.LookPath,
			shellsFile,
		)
		if err != nil {
			t.Fatalf("BuildConsoleLaunchPlan failed: %v", err)
		}

		var stdout bytes.Buffer
		code, err := runSupervisedSession(SupervisedSessionSpec{
			Binary: plan.Binary,
			Args:   plan.Args,
			Env:    []string{"HOME=" + resolvedHome, "PATH=/usr/bin:/bin"},
			Dir:    resolvedHome,
			Stdin:  strings.NewReader("shopt -q login_shell && echo LOGIN || echo NONLOGIN\nexit 0\n"),
			Stdout: &stdout,
			Stderr: &bytes.Buffer{},
		})
		if err != nil || code != 0 {
			t.Fatalf("got (%d, %v), want (0, nil)", code, err)
		}
		if !strings.Contains(stdout.String(), "NONLOGIN") {
			t.Fatalf("expected NONLOGIN in user mode, got %q", stdout.String())
		}
		if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
			t.Fatalf(".bash_profile must not run in user mode (marker exists: %v)", statErr)
		}
	})
}

func TestRealZshLoginAndUserModes(t *testing.T) {
	zshBin, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh not installed")
	}

	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedHome, err := filepath.EvalSymlinks(homeDir)
	if err != nil {
		t.Fatal(err)
	}

	shellsFile := filepath.Join(tmpDir, "shells")
	if err := os.WriteFile(shellsFile, []byte(zshBin+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	acct := &config.AccountInfo{
		Username: "tester",
		UID:      os.Geteuid(),
		GID:      os.Getegid(),
		HomeDir:  resolvedHome,
		Shell:    "/bin/sh",
	}

	t.Run("getty mode launches real zsh as login shell with zshenv zprofile zlogin", func(t *testing.T) {
		orderFile := filepath.Join(tmpDir, "zsh_order.log")
		if err := os.WriteFile(filepath.Join(resolvedHome, ".zshenv"), []byte(fmt.Sprintf("echo zshenv >> %q\n", orderFile)), 0o644); err != nil {
			t.Fatal(err)
		}
		zprofile := fmt.Sprintf(
			"echo zprofile >> %q\n[[ -o login ]] && echo LOGIN=yes >> %q || echo LOGIN=no >> %q\nprintf 'ARG0=%%s\\nPWD=%%s\\nHOME=%%s\\nSHELL=%%s\\nSPAWN=%%s\\n' \"$0\" \"$PWD\" \"$HOME\" \"$SHELL\" \"$GOCDM_SPAWN\" >> %q\n",
			orderFile, orderFile, orderFile, orderFile,
		)
		if err := os.WriteFile(filepath.Join(resolvedHome, ".zprofile"), []byte(zprofile), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(resolvedHome, ".zlogin"), []byte(fmt.Sprintf("echo zlogin >> %q\n", orderFile)), 0o644); err != nil {
			t.Fatal(err)
		}

		plan, err := BuildConsoleLaunchPlan(
			session.Session{Name: "zsh", Exec: zshBin, Type: "C", IsShell: true},
			ModeGettyLogin,
			exec.LookPath,
			shellsFile,
		)
		if err != nil {
			t.Fatalf("BuildConsoleLaunchPlan failed: %v", err)
		}

		env, err := config.BuildGettySessionEnv(config.GettyEnvOptions{
			Account:     acct,
			SessionType: "C",
			SessionName: "zsh",
			WorkingDir:  resolvedHome,
			SpawnPID:    os.Getpid(),
		})
		if err != nil {
			t.Fatalf("BuildGettySessionEnv failed: %v", err)
		}
		env = config.SetEnvValue(env, "SHELL", plan.ShellEnv)

		code, err := runSupervisedSession(SupervisedSessionSpec{
			Binary: plan.Binary,
			Args:   plan.Args,
			Env:    env,
			Dir:    resolvedHome,
			Stdin:  strings.NewReader("exit 19\n"),
			Stdout: &bytes.Buffer{},
			Stderr: &bytes.Buffer{},
		})
		if err != nil {
			t.Fatalf("runSupervisedSession failed: %v", err)
		}
		if code != 19 {
			t.Fatalf("exit code = %d, want 19", code)
		}

		data, err := os.ReadFile(orderFile)
		if err != nil {
			t.Fatalf("read zsh order log: %v", err)
		}
		text := string(data)
		for _, want := range []string{
			"zshenv",
			"zprofile",
			"LOGIN=yes",
			"ARG0=-" + filepath.Base(zshBin),
			"PWD=" + resolvedHome,
			"HOME=" + resolvedHome,
			"SHELL=" + zshBin,
			fmt.Sprintf("SPAWN=%d", os.Getpid()),
			"zlogin",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("expected %q in zsh log, got:\n%s", want, text)
			}
		}
		idxEnv := strings.Index(text, "zshenv")
		idxProf := strings.Index(text, "zprofile")
		idxLogin := strings.Index(text, "zlogin")
		if !(idxEnv < idxProf && idxProf < idxLogin) {
			t.Fatalf("unexpected zsh startup order in:\n%s", text)
		}
	})

	t.Run("interactive login zsh runs zshenv zprofile zshrc zlogin zlogout in order", func(t *testing.T) {
		orderFile := filepath.Join(tmpDir, "zsh_interactive_order.log")
		if err := os.WriteFile(filepath.Join(resolvedHome, ".zshenv"), []byte(fmt.Sprintf("echo zshenv >> %q\n", orderFile)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(resolvedHome, ".zprofile"), []byte(fmt.Sprintf("echo zprofile >> %q\n", orderFile)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(resolvedHome, ".zshrc"), []byte(fmt.Sprintf("echo zshrc >> %q\n", orderFile)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(resolvedHome, ".zlogin"), []byte(fmt.Sprintf("echo zlogin >> %q\n", orderFile)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(resolvedHome, ".zlogout"), []byte(fmt.Sprintf("echo zlogout >> %q\n", orderFile)), 0o644); err != nil {
			t.Fatal(err)
		}

		plan, err := BuildConsoleLaunchPlan(
			session.Session{Name: "zsh", Exec: zshBin, Type: "C", IsShell: true},
			ModeGettyLogin,
			exec.LookPath,
			shellsFile,
		)
		if err != nil {
			t.Fatalf("BuildConsoleLaunchPlan failed: %v", err)
		}

		env, err := config.BuildGettySessionEnv(config.GettyEnvOptions{
			Account:     acct,
			SessionType: "C",
			SessionName: "zsh",
			WorkingDir:  resolvedHome,
			SpawnPID:    os.Getpid(),
		})
		if err != nil {
			t.Fatalf("BuildGettySessionEnv failed: %v", err)
		}
		env = config.SetEnvValue(env, "SHELL", plan.ShellEnv)

		code, err := runSupervisedSession(SupervisedSessionSpec{
			Binary: plan.Binary,
			Args:   append(plan.Args, "-ic", "exit 0"),
			Env:    env,
			Dir:    resolvedHome,
			Stdin:  strings.NewReader(""),
			Stdout: &bytes.Buffer{},
			Stderr: &bytes.Buffer{},
		})
		if err != nil || code != 0 {
			t.Fatalf("interactive login zsh failed: (%d, %v)", code, err)
		}

		data, err := os.ReadFile(orderFile)
		if err != nil {
			t.Fatalf("read interactive zsh order log: %v", err)
		}
		got := strings.Fields(string(data))
		want := []string{"zshenv", "zprofile", "zshrc", "zlogin", "zlogout"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("zsh interactive login sequence = %v, want %v", got, want)
		}
	})

	t.Run("user mode does not invoke zsh login startup scripts", func(t *testing.T) {
		userLog := filepath.Join(tmpDir, "zsh_user.log")
		if err := os.WriteFile(filepath.Join(resolvedHome, ".zprofile"), []byte(fmt.Sprintf("echo zprofile >> %q\n", userLog)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(resolvedHome, ".zlogin"), []byte(fmt.Sprintf("echo zlogin >> %q\n", userLog)), 0o644); err != nil {
			t.Fatal(err)
		}

		plan, err := BuildConsoleLaunchPlan(
			session.Session{Name: "zsh", Exec: zshBin, Type: "C", IsShell: true},
			ModeUserSession,
			exec.LookPath,
			shellsFile,
		)
		if err != nil {
			t.Fatalf("BuildConsoleLaunchPlan failed: %v", err)
		}

		var stdout bytes.Buffer
		code, err := runSupervisedSession(SupervisedSessionSpec{
			Binary: plan.Binary,
			Args:   plan.Args,
			Env:    []string{"HOME=" + resolvedHome, "PATH=/usr/bin:/bin"},
			Dir:    resolvedHome,
			Stdin:  strings.NewReader("[[ -o login ]] && echo LOGIN || echo NONLOGIN\nexit 0\n"),
			Stdout: &stdout,
			Stderr: &bytes.Buffer{},
		})
		if err != nil || code != 0 {
			t.Fatalf("got (%d, %v), want (0, nil)", code, err)
		}
		if !strings.Contains(stdout.String(), "NONLOGIN") {
			t.Fatalf("expected NONLOGIN in user mode, got %q", stdout.String())
		}
		if data, err := os.ReadFile(userLog); err == nil {
			t.Fatalf("zprofile/zlogin must not run in user mode, got %q", string(data))
		}
	})
}

func TestCompiledArbitraryConsoleCommandDoesNotSourceShellProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedHome, err := filepath.EvalSymlinks(homeDir)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(resolvedHome, ".bash_profile"), []byte("touch \"$HOME/.bash_trap_ran\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resolvedHome, ".zprofile"), []byte("touch \"$HOME/.zsh_trap_ran\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	shellsFile := filepath.Join(tmpDir, "shells")
	if err := os.WriteFile(shellsFile, []byte("/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmdStr := fmt.Sprintf("%s -test.run=^TestConsoleArbitraryProbe$ -- \"hello world\"", os.Args[0])
	plan, err := BuildConsoleLaunchPlan(
		session.Session{Name: "Probe", Exec: cmdStr, Type: "C"},
		ModeGettyLogin,
		func(file string) (string, error) { return file, nil },
		shellsFile,
	)
	if err != nil {
		t.Fatalf("BuildConsoleLaunchPlan failed: %v", err)
	}
	if plan.IsLoginShell {
		t.Fatalf("arbitrary command must not be marked as login shell: %+v", plan)
	}

	acct := &config.AccountInfo{
		Username: "tester",
		UID:      os.Geteuid(),
		GID:      os.Getegid(),
		HomeDir:  resolvedHome,
		Shell:    "/bin/bash",
	}
	env, err := config.BuildGettySessionEnv(config.GettyEnvOptions{
		Account:     acct,
		SessionType: "C",
		SessionName: "Probe",
		WorkingDir:  resolvedHome,
	})
	if err != nil {
		t.Fatalf("BuildGettySessionEnv failed: %v", err)
	}
	env = append(env, "GO_WANT_CONSOLE_PROBE=1")

	var stdout bytes.Buffer
	code, err := runSupervisedSession(SupervisedSessionSpec{
		Binary: plan.Binary,
		Args:   plan.Args,
		Env:    env,
		Dir:    resolvedHome,
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
	})
	if err != nil || code != 0 {
		t.Fatalf("arbitrary probe failed: (%d, %v)", code, err)
	}

	var res consoleProbeResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal probe %q: %v", stdout.String(), err)
	}
	if strings.HasPrefix(filepath.Base(res.Arg0), "-") {
		t.Fatalf("arbitrary command argv[0] must not have leading '-': %q", res.Arg0)
	}
	if len(res.Args) != 3 || res.Args[2] != "hello world" {
		t.Fatalf("expected quoted argument preserved as single argv item, got %q", res.Args)
	}
	if res.CWD != resolvedHome || res.Home != resolvedHome || res.Shell != "/bin/bash" || !res.TrapOK {
		t.Fatalf("unexpected arbitrary command probe result: %+v", res)
	}
}

func TestCliRunConsoleShellGettyVsUserMode(t *testing.T) {
	t.Run("getty mode launches discovered zsh with login argv[0] and updates SHELL", func(t *testing.T) {
		setupGettyContextTestEnv(t)

		tmpDir := t.TempDir()
		aliceHome := filepath.Join(tmpDir, "alice")
		if err := os.MkdirAll(aliceHome, 0o755); err != nil {
			t.Fatal(err)
		}
		passwdPath := filepath.Join(tmpDir, "passwd")
		uid := os.Geteuid()
		if err := os.WriteFile(passwdPath, []byte(fmt.Sprintf("alice:x:%d:%d:Alice:%s:/bin/bash\n", uid, uid, aliceHome)), 0o644); err != nil {
			t.Fatal(err)
		}
		pamEnvPath := filepath.Join(tmpDir, "pam_env.conf")
		if err := os.WriteFile(pamEnvPath, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		PasswdFilePath = passwdPath
		PamEnvConfPath = pamEnvPath

		shellsPath := filepath.Join(tmpDir, "shells")
		if err := os.WriteFile(shellsPath, []byte("/usr/bin/zsh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		origShells := ShellsFilePath
		ShellsFilePath = shellsPath
		t.Cleanup(func() { ShellsFilePath = origShells })

		DiscoverSessionsFn = func(userHome string) ([]session.Session, error) {
			return []session.Session{{Name: "zsh", Exec: "/usr/bin/zsh", Type: "C", Path: shellsPath, IsShell: true}}, nil
		}
		var closed bool
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{session: &spyLoginSession{username: "alice", closeCalled: &closed}}
		}
		TuiPromptCredentials = func(title, theme, version string, authFunc func(string, string) error) (string, string, error) {
			return "alice", "pw", authFunc("alice", "pw")
		}
		ExecLookPath = func(file string) (string, error) { return "/usr/bin/zsh", nil }
		SaveStateAsUserFn = func(acct *config.AccountInfo, lastSession string) error { return nil }

		var gotSpec SupervisedSessionSpec
		RunSupervisedSessionFn = func(spec SupervisedSessionSpec) (int, error) {
			gotSpec = spec
			return 0, nil
		}

		var exited bool
		var code int
		Run([]string{"-login"}, func(c int) { exited = true; code = c })
		if !exited || code != 0 || !closed {
			t.Fatalf("expected exit(0) and PAM close, got exited=%v code=%d closed=%v", exited, code, closed)
		}
		if gotSpec.Binary != "/usr/bin/zsh" || !reflect.DeepEqual(gotSpec.Args, []string{"-zsh"}) {
			t.Fatalf("expected login zsh invocation, got binary=%q args=%v", gotSpec.Binary, gotSpec.Args)
		}
		envMap := config.EnvSliceToMap(gotSpec.Env)
		if envMap["SHELL"] != "/usr/bin/zsh" {
			t.Fatalf("expected SHELL=/usr/bin/zsh for selected shell session, got %q", envMap["SHELL"])
		}
	})

	t.Run("user mode launches discovered zsh without login argv[0]", func(t *testing.T) {
		setupGettyContextTestEnv(t)
		osGetuid = func() int { return 1000 }
		osGeteuid = func() int { return 1000 }

		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		shellsPath := filepath.Join(tmpDir, "shells")
		if err := os.WriteFile(shellsPath, []byte("/usr/bin/zsh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		origShells := ShellsFilePath
		ShellsFilePath = shellsPath
		t.Cleanup(func() { ShellsFilePath = origShells })

		DiscoverSessionsFn = func(userHome string) ([]session.Session, error) {
			return []session.Session{{Name: "zsh", Exec: "/usr/bin/zsh", Type: "C", Path: shellsPath, IsShell: true}}, nil
		}
		ExecLookPath = func(file string) (string, error) { return "/usr/bin/zsh", nil }

		var gotBinary string
		var gotArgs []string
		ExecProgramFn = func(binary string, args []string, env []string) error {
			gotBinary = binary
			gotArgs = args
			return nil
		}

		Run([]string{"-mode=user"}, func(c int) { t.Fatalf("unexpected exit(%d)", c) })
		if gotBinary != "/usr/bin/zsh" || !reflect.DeepEqual(gotArgs, []string{"/usr/bin/zsh"}) {
			t.Fatalf("expected non-login zsh invocation in user mode, got binary=%q args=%v", gotBinary, gotArgs)
		}
	})
}
