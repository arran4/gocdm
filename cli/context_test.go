package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/arran4/gocdm/auth"
	"github.com/arran4/gocdm/config"
	"github.com/arran4/gocdm/session"
)

type spyLoginSession struct {
	events      *[]string
	pamEnv      []string
	openErr     error
	closeCalled *bool
}

func (s *spyLoginSession) OpenSession() error {
	if s.events != nil {
		*s.events = append(*s.events, "open_session")
	}
	return s.openErr
}

func (s *spyLoginSession) Env() []string {
	return s.pamEnv
}

func (s *spyLoginSession) CloseSession() error {
	if s.closeCalled != nil {
		*s.closeCalled = true
	}
	return nil
}

type spyAuthenticator struct {
	events  *[]string
	session auth.LoginSession
}

func (a spyAuthenticator) Authenticate(username, password string) (auth.LoginSession, error) {
	if a.events != nil {
		*a.events = append(*a.events, "auth")
	}
	return a.session, nil
}

func setupGettyContextTestEnv(t *testing.T) {
	t.Helper()
	mockUnsupported(t)
	origGetuid := osGetuid
	origGeteuid := osGeteuid
	origIsTerminal := IsTerminal
	origNewAuth := NewAuthenticator
	origTuiPrompt := TuiPromptCredentials
	origPrompt := PromptCredentials
	origLookPath := ExecLookPath
	origDrop := DropPrivilegesFn
	origExec := ExecProgramFn
	origDiscover := DiscoverSessionsFn
	origLookup := LookupAccountFn
	origChdir := ChdirFn
	origSaveState := SaveStateFn
	origPasswd := PasswdFilePath
	origPamEnv := PamEnvConfPath
	origSysCfg := config.SystemConfigPath

	t.Cleanup(func() {
		osGetuid = origGetuid
		osGeteuid = origGeteuid
		IsTerminal = origIsTerminal
		NewAuthenticator = origNewAuth
		TuiPromptCredentials = origTuiPrompt
		PromptCredentials = origPrompt
		ExecLookPath = origLookPath
		DropPrivilegesFn = origDrop
		ExecProgramFn = origExec
		DiscoverSessionsFn = origDiscover
		LookupAccountFn = origLookup
		ChdirFn = origChdir
		SaveStateFn = origSaveState
		PasswdFilePath = origPasswd
		PamEnvConfPath = origPamEnv
		config.SystemConfigPath = origSysCfg
	})

	osGetuid = func() int { return 0 }
	osGeteuid = func() int { return 0 }
	IsTerminal = func(int) bool { return true }
	for _, k := range []string{"DISPLAY", "WAYLAND_DISPLAY", "SSH_TTY", "SSH_CONNECTION", "SSH_CLIENT"} {
		t.Setenv(k, "")
	}
}

func TestGettyLoginUsesAuthenticatedUserConfigAndSessionsNotRoots(t *testing.T) {
	setupGettyContextTestEnv(t)

	tmpDir := t.TempDir()
	rootHome := filepath.Join(tmpDir, "root-home")
	aliceHome := filepath.Join(tmpDir, "alice-home")
	if err := os.MkdirAll(rootHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(aliceHome, 0o755); err != nil {
		t.Fatal(err)
	}

	// Root's home has a config and .xinitrc that must NEVER be selected for alice.
	if err := os.WriteFile(filepath.Join(rootHome, ".cdmrc"), []byte("binlist=(\"root-cmd\")\nnamelist=(\"Root Cmd\")\nflaglist=(\"C\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootHome, ".xinitrc"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	sysCfg := filepath.Join(tmpDir, "etc-cdmrc")
	if err := os.WriteFile(sysCfg, []byte("dialogrc=\"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config.SystemConfigPath = sysCfg

	// Alice uses her own .cdmrc with empty binlist so session discovery runs against aliceHome.
	if err := os.WriteFile(filepath.Join(aliceHome, ".cdmrc"), []byte("countfrom=2\n"), 0o644); err != nil {
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

	// Launcher environment points HOME at rootHome with a secret that must not leak.
	t.Setenv("HOME", rootHome)
	t.Setenv("USER", "root")
	t.Setenv("LEAKED_SECRET", "top-secret-value")

	var discoveredHome string
	DiscoverSessionsFn = func(userHome string) ([]session.Session, error) {
		discoveredHome = userHome
		return []session.Session{{Name: "alice-zsh", Exec: "/bin/zsh", Type: "C"}}, nil
	}

	NewAuthenticator = func(string) auth.Authenticator {
		return spyAuthenticator{session: &spyLoginSession{pamEnv: []string{"XDG_RUNTIME_DIR=/run/user/1001"}}}
	}
	TuiPromptCredentials = func(title, theme, version string, authFunc func(string, string) error) (string, string, error) {
		return "alice", "pw", authFunc("alice", "pw")
	}

	ExecLookPath = func(file string) (string, error) { return file, nil }
	DropPrivilegesFn = func(username string) error {
		if username != "alice" {
			return fmt.Errorf("unexpected user %q", username)
		}
		return nil
	}
	var chdirTarget string
	ChdirFn = func(dir string) error {
		chdirTarget = dir
		return nil
	}
	var savedHome, savedSession string
	SaveStateFn = func(home, lastSession string, expectedUID int) error {
		savedHome = home
		savedSession = lastSession
		if expectedUID != uid {
			return fmt.Errorf("expectedUID=%d want %d", expectedUID, uid)
		}
		return nil
	}

	var capturedEnv []string
	ExecProgramFn = func(binary string, args []string, env []string) error {
		capturedEnv = env
		return nil
	}

	Run([]string{"-login"}, func(c int) { t.Fatalf("unexpected exit(%d)", c) })

	if discoveredHome != aliceHome {
		t.Fatalf("DiscoverSessionsFn called with %q, want %q (not rootHome %q)", discoveredHome, aliceHome, rootHome)
	}
	if chdirTarget != aliceHome {
		t.Fatalf("ChdirFn target = %q, want %q", chdirTarget, aliceHome)
	}
	if savedHome != aliceHome || savedSession != "alice-zsh" {
		t.Fatalf("SaveStateFn got (%q, %q), want (%q, %q)", savedHome, savedSession, aliceHome, "alice-zsh")
	}
	envMap := config.EnvSliceToMap(capturedEnv)
	if envMap["HOME"] != aliceHome || envMap["USER"] != "alice" || envMap["LOGNAME"] != "alice" || envMap["SHELL"] != "/bin/zsh" {
		t.Fatalf("unexpected identity vars in exec env: %v", envMap)
	}
	if _, leaked := envMap["LEAKED_SECRET"]; leaked {
		t.Fatal("LEAKED_SECRET from launcher env leaked into authenticated session")
	}
	if envMap["XDG_RUNTIME_DIR"] != "/run/user/1001" {
		t.Fatalf("expected PAM XDG_RUNTIME_DIR, got %q", envMap["XDG_RUNTIME_DIR"])
	}
}

func TestGettyLoginPrivilegeAndStateOrdering(t *testing.T) {
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

	cfgPath := filepath.Join(tmpDir, "cdmrc")
	if err := os.WriteFile(cfgPath, []byte("binlist=(\"echo ok\")\nnamelist=(\"Shell\")\nflaglist=(\"C\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var events []string
	NewAuthenticator = func(string) auth.Authenticator {
		return spyAuthenticator{events: &events, session: &spyLoginSession{events: &events}}
	}
	TuiPromptCredentials = func(title, theme, version string, authFunc func(string, string) error) (string, string, error) {
		return "alice", "pw", authFunc("alice", "pw")
	}
	ExecLookPath = func(file string) (string, error) { return "/bin/" + file, nil }
	DropPrivilegesFn = func(username string) error {
		events = append(events, "drop_privileges")
		return nil
	}
	ChdirFn = func(dir string) error {
		events = append(events, "chdir")
		return nil
	}
	SaveStateFn = func(home, lastSession string, expectedUID int) error {
		events = append(events, "save_state")
		return nil
	}
	ExecProgramFn = func(binary string, args []string, env []string) error {
		events = append(events, "exec")
		return nil
	}

	Run([]string{"-login", "-config", cfgPath}, func(c int) { t.Fatalf("unexpected exit(%d)", c) })

	want := []string{"auth", "open_session", "drop_privileges", "chdir", "save_state", "exec"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("call order = %v, want %v", events, want)
	}
}

func TestGettyLoginMissingHomeFallsBackToRootDir(t *testing.T) {
	setupGettyContextTestEnv(t)

	tmpDir := t.TempDir()
	missingHome := filepath.Join(tmpDir, "does-not-exist")
	passwdPath := filepath.Join(tmpDir, "passwd")
	if err := os.WriteFile(passwdPath, []byte(fmt.Sprintf("alice:x:1001:1001:Alice:%s:/bin/bash\n", missingHome)), 0o644); err != nil {
		t.Fatal(err)
	}
	pamEnvPath := filepath.Join(tmpDir, "pam_env.conf")
	if err := os.WriteFile(pamEnvPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	PasswdFilePath = passwdPath
	PamEnvConfPath = pamEnvPath

	cfgPath := filepath.Join(tmpDir, "cdmrc")
	if err := os.WriteFile(cfgPath, []byte("binlist=(\"echo ok\")\nnamelist=(\"Echo\")\nflaglist=(\"C\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	NewAuthenticator = func(string) auth.Authenticator {
		return spyAuthenticator{session: &spyLoginSession{}}
	}
	TuiPromptCredentials = func(title, theme, version string, authFunc func(string, string) error) (string, string, error) {
		return "alice", "pw", authFunc("alice", "pw")
	}
	ExecLookPath = func(file string) (string, error) { return "/bin/" + file, nil }
	DropPrivilegesFn = func(string) error { return nil }

	var chdirCalls []string
	ChdirFn = func(dir string) error {
		chdirCalls = append(chdirCalls, dir)
		if dir == missingHome {
			return os.ErrNotExist
		}
		return nil
	}
	SaveStateFn = func(home, lastSession string, expectedUID int) error {
		return os.ErrNotExist
	}

	var capturedEnv []string
	ExecProgramFn = func(binary string, args []string, env []string) error {
		capturedEnv = env
		return nil
	}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	Run([]string{"-login", "-config", cfgPath}, func(c int) { t.Fatalf("unexpected exit(%d)", c) })
	_ = w.Close()
	errOut, _ := io.ReadAll(r)

	if !reflect.DeepEqual(chdirCalls, []string{missingHome, "/"}) {
		t.Fatalf("chdir calls = %v, want [%s /]", chdirCalls, missingHome)
	}
	if !strings.Contains(string(errOut), "falling back to /") {
		t.Fatalf("expected fallback warning on stderr, got %q", string(errOut))
	}
	envMap := config.EnvSliceToMap(capturedEnv)
	if envMap["PWD"] != "/" {
		t.Fatalf("expected PWD=/ after fallback, got %q", envMap["PWD"])
	}
}

func TestGettyLoginFailuresCloseSessionAndNeverFallbackToRootEnv(t *testing.T) {
	t.Run("account lookup failure closes PAM session", func(t *testing.T) {
		setupGettyContextTestEnv(t)
		tmpDir := t.TempDir()
		passwdPath := filepath.Join(tmpDir, "passwd")
		if err := os.WriteFile(passwdPath, []byte("root:x:0:0:root:/root:/bin/sh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		PasswdFilePath = passwdPath
		cfgPath := filepath.Join(tmpDir, "cdmrc")
		if err := os.WriteFile(cfgPath, []byte("binlist=(\"echo ok\")\nnamelist=(\"Echo\")\nflaglist=(\"C\")\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		var closed bool
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{session: &spyLoginSession{closeCalled: &closed}}
		}
		TuiPromptCredentials = func(title, theme, version string, authFunc func(string, string) error) (string, string, error) {
			return "ghost", "pw", authFunc("ghost", "pw")
		}
		ExecProgramFn = func(string, []string, []string) error {
			t.Fatal("ExecProgramFn must not be called when account lookup fails")
			return nil
		}

		exited := false
		code := -1
		Run([]string{"-login", "-config", cfgPath}, func(c int) { exited = true; code = c })
		if !exited || code != 1 {
			t.Fatalf("expected exit(1), got exited=%v code=%d", exited, code)
		}
		if !closed {
			t.Fatal("expected PAM login session to be closed on account lookup failure")
		}
	})

	t.Run("session env build failure exits instead of falling back to os.Environ", func(t *testing.T) {
		setupGettyContextTestEnv(t)
		tmpDir := t.TempDir()
		aliceHome := filepath.Join(tmpDir, "alice")
		if err := os.MkdirAll(aliceHome, 0o755); err != nil {
			t.Fatal(err)
		}
		passwdPath := filepath.Join(tmpDir, "passwd")
		if err := os.WriteFile(passwdPath, []byte(fmt.Sprintf("alice:x:1001:1001:Alice:%s:/bin/bash\n", aliceHome)), 0o644); err != nil {
			t.Fatal(err)
		}
		PasswdFilePath = passwdPath
		// Point PamEnvConfPath to a directory so BuildGettySessionEnv fails.
		PamEnvConfPath = tmpDir

		cfgPath := filepath.Join(tmpDir, "cdmrc")
		if err := os.WriteFile(cfgPath, []byte("binlist=(\"echo ok\")\nnamelist=(\"Echo\")\nflaglist=(\"C\")\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		var closed bool
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{session: &spyLoginSession{closeCalled: &closed}}
		}
		TuiPromptCredentials = func(title, theme, version string, authFunc func(string, string) error) (string, string, error) {
			return "alice", "pw", authFunc("alice", "pw")
		}
		ExecLookPath = func(file string) (string, error) { return "/bin/" + file, nil }
		ExecProgramFn = func(string, []string, []string) error {
			t.Fatal("ExecProgramFn must not be called when env construction fails")
			return nil
		}

		exited := false
		code := -1
		Run([]string{"-login", "-config", cfgPath}, func(c int) { exited = true; code = c })
		if !exited || code != 1 {
			t.Fatalf("expected exit(1), got exited=%v code=%d", exited, code)
		}
		if !closed {
			t.Fatal("expected PAM session closed when env build fails")
		}
	})
}

func TestUserSessionModePreservesInheritedEnvAndCWD(t *testing.T) {
	setupGettyContextTestEnv(t)
	uid := os.Getuid()
	if uid == 0 {
		uid = 1000
	}
	osGetuid = func() int { return uid }
	osGeteuid = func() int { return uid }

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USER_CUSTOM_TOKEN", "keep-me")

	cfgPath := filepath.Join(tmpDir, "cdmrc")
	if err := os.WriteFile(cfgPath, []byte("binlist=(\"echo ok\")\nnamelist=(\"Echo\")\nflaglist=(\"C\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ChdirFn = func(string) error {
		t.Fatal("ChdirFn must not be called in already-authenticated user mode")
		return nil
	}
	DropPrivilegesFn = func(string) error {
		t.Fatal("DropPrivilegesFn must not be called in user mode")
		return nil
	}
	ExecLookPath = func(file string) (string, error) { return "/bin/" + file, nil }

	var capturedEnv []string
	ExecProgramFn = func(binary string, args []string, env []string) error {
		capturedEnv = env
		return nil
	}

	Run([]string{"-mode=user", "-config", cfgPath}, func(c int) { t.Fatalf("unexpected exit(%d)", c) })

	envMap := config.EnvSliceToMap(capturedEnv)
	if envMap["USER_CUSTOM_TOKEN"] != "keep-me" {
		t.Fatalf("expected USER_CUSTOM_TOKEN preserved, got %q", envMap["USER_CUSTOM_TOKEN"])
	}
	state, err := config.LoadStateAt(tmpDir)
	if err != nil || state.LastSession != "Echo" {
		t.Fatalf("expected saved state 'Echo' in %s, got state=%+v err=%v", tmpDir, state, err)
	}
}
