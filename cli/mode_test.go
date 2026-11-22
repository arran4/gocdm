package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/arran4/gocdm/auth"
	"github.com/arran4/gocdm/config"
)

func TestParseModeFlag(t *testing.T) {
	tests := []struct {
		raw     string
		want    ModeSelection
		wantErr bool
	}{
		{"", ModeAuto, false},
		{"auto", ModeAuto, false},
		{" AUTO ", ModeAuto, false},
		{"login", ModeSelectLogin, false},
		{"Getty", ModeSelectLogin, false},
		{"user", ModeSelectUser, false},
		{"autostart", ModeSelectUser, false},
		{"SESSION", ModeSelectUser, false},
		{"bogus", "", true},
		{"daemon", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := ParseModeFlag(tc.raw)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseModeFlag(%q) err = %v, wantErr=%v", tc.raw, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("ParseModeFlag(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestResolveModeSelection(t *testing.T) {
	tests := []struct {
		name         string
		modeFlagSet  bool
		modeRaw      string
		loginFlagSet bool
		loginVal     bool
		want         ModeSelection
		wantErr      string
	}{
		{"defaults", false, "auto", false, false, ModeAuto, ""},
		{"explicit mode login", true, "login", false, false, ModeSelectLogin, ""},
		{"explicit mode getty", true, "getty", false, false, ModeSelectLogin, ""},
		{"explicit mode user", true, "user", false, false, ModeSelectUser, ""},
		{"explicit mode autostart", true, "autostart", false, false, ModeSelectUser, ""},
		{"explicit login true", false, "auto", true, true, ModeSelectLogin, ""},
		{"explicit login false", false, "auto", true, false, ModeSelectUser, ""},
		{"compatible login true and mode auto", true, "auto", true, true, ModeSelectLogin, ""},
		{"compatible login true and mode getty", true, "getty", true, true, ModeSelectLogin, ""},
		{"compatible login false and mode user", true, "user", true, false, ModeSelectUser, ""},
		{"conflicting login true and mode user", true, "user", true, true, "", "conflicting mode flags"},
		{"conflicting login false and mode login", true, "login", true, false, "", "conflicting mode flags"},
		{"invalid mode", true, "invalid", false, false, "", "invalid -mode"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveModeSelection(tc.modeFlagSet, tc.modeRaw, tc.loginFlagSet, tc.loginVal)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func cleanGettyContext() ProcessContext {
	return ProcessContext{
		UID:           0,
		EUID:          0,
		GOOS:          "linux",
		SupportTier:   "primary",
		SupportsLogin: true,
		Getenv:        func(string) string { return "" },
		Readlink: func(p string) (string, error) {
			if p == "/proc/self/fd/0" || p == "/proc/self/fd/1" {
				return "/dev/tty1", nil
			}
			return "", os.ErrNotExist
		},
		Stat: func(p string) (os.FileInfo, error) {
			if p == "/dev/tty1" {
				return mockFileInfo{mode: os.ModeCharDevice}, nil
			}
			return nil, os.ErrNotExist
		},
		IsTerminal:   func(int) bool { return true },
		ReadLoginUID: func() (uint32, bool) { return unsetLoginUID, true },
	}
}

func envLookup(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolveOperatingMode(t *testing.T) {
	tests := []struct {
		name     string
		sel      ModeSelection
		setupCtx func() ProcessContext
		dryRun   bool
		want     OperatingMode
		wantErr  string
	}{
		{
			name:     "real getty-style fixture auto-detects login",
			sel:      ModeAuto,
			setupCtx: cleanGettyContext,
			want:     ModeGettyLogin,
		},
		{
			name: "post-login non-root user on tty1 with empty DISPLAY stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.UID = 1000
				ctx.EUID = 1000
				ctx.ReadLoginUID = func() (uint32, bool) { return 1000, true }
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "root with unset UID but non-zero EUID stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.UID = 1000
				ctx.EUID = 0
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "post-login root on tty1 with USER set stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"USER": "root"})
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "post-login root on tty1 with LOGNAME set stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"LOGNAME": "root"})
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "post-login root on tty1 with SHLVL set stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"SHLVL": "1"})
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "post-login root on tty1 with XDG_SESSION_ID set stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"XDG_SESSION_ID": "3"})
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "post-login root on tty1 with GOCDM_SPAWN set stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"GOCDM_SPAWN": "42"})
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "post-login root on tty1 with SUDO_USER set stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"SUDO_USER": "alice"})
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "post-login root on tty1 with DOAS_USER set stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"DOAS_USER": "alice"})
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "root on tty1 with DISPLAY set stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"DISPLAY": ":0"})
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "root on tty1 with WAYLAND_DISPLAY set stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"WAYLAND_DISPLAY": "wayland-0"})
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "post-login root on tty1 with audit loginuid set stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.ReadLoginUID = func() (uint32, bool) { return 0, true }
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "root with mismatched stdin tty1 and stdout pts0 stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Readlink = func(p string) (string, error) {
					if p == "/proc/self/fd/0" {
						return "/dev/tty1", nil
					}
					if p == "/proc/self/fd/1" {
						return "/dev/pts/0", nil
					}
					return "", os.ErrNotExist
				}
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name: "root on SSH PTY stays user mode",
			sel:  ModeAuto,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"SSH_TTY": "/dev/pts/0"})
				ctx.Readlink = func(p string) (string, error) { return "/dev/pts/0", nil }
				return ctx
			},
			want: ModeUserSession,
		},
		{
			name:     "explicit user mode overrides getty context",
			sel:      ModeSelectUser,
			setupCtx: cleanGettyContext,
			want:     ModeUserSession,
		},
		{
			name:     "explicit login mode in clean root context",
			sel:      ModeSelectLogin,
			setupCtx: cleanGettyContext,
			want:     ModeGettyLogin,
		},
		{
			name: "explicit login mode as non-root fails clearly",
			sel:  ModeSelectLogin,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.UID = 1000
				ctx.EUID = 1000
				return ctx
			},
			wantErr: "requires root privileges",
		},
		{
			name: "explicit login mode inside X11 DISPLAY fails",
			sel:  ModeSelectLogin,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"DISPLAY": ":0"})
				return ctx
			},
			wantErr: "active graphical session",
		},
		{
			name: "explicit login mode inside WAYLAND_DISPLAY fails",
			sel:  ModeSelectLogin,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"WAYLAND_DISPLAY": "wayland-0"})
				return ctx
			},
			wantErr: "active graphical session",
		},
		{
			name: "explicit login mode inside SSH_TTY fails",
			sel:  ModeSelectLogin,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"SSH_TTY": "/dev/pts/0"})
				return ctx
			},
			wantErr: "inside an SSH session",
		},
		{
			name: "explicit login mode inside SSH_CONNECTION fails",
			sel:  ModeSelectLogin,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"SSH_CONNECTION": "10.0.0.1 1234 10.0.0.2 22"})
				return ctx
			},
			wantErr: "inside an SSH session",
		},
		{
			name: "explicit login mode inside SSH_CLIENT fails",
			sel:  ModeSelectLogin,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.Getenv = envLookup(map[string]string{"SSH_CLIENT": "10.0.0.1 1234 22"})
				return ctx
			},
			wantErr: "inside an SSH session",
		},
		{
			name: "explicit login mode on unsupported platform fails",
			sel:  ModeSelectLogin,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.GOOS = "windows"
				ctx.SupportTier = "unsupported"
				ctx.SupportsLogin = false
				return ctx
			},
			wantErr: "-login is not supported on windows (support tier: unsupported)",
		},
		{
			name: "explicit login mode in dry-run allows non-root",
			sel:  ModeSelectLogin,
			setupCtx: func() ProcessContext {
				ctx := cleanGettyContext()
				ctx.UID = 1000
				ctx.EUID = 1000
				return ctx
			},
			dryRun: true,
			want:   ModeGettyLogin,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := ResolveOperatingMode(tc.sel, tc.setupCtx(), tc.dryRun)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got mode %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunPostLoginAutostartOnTTY1NeverPromptsPAM(t *testing.T) {
	mockEnvFiles(t)
	mockUnsupported(t)

	origReadlink := osReadlink
	origStat := osStat
	origGetuid := osGetuid
	origGeteuid := osGeteuid
	origIsTerminal := IsTerminal
	origNewAuth := NewAuthenticator
	origPrompt := PromptCredentials
	origTuiPrompt := TuiPromptCredentials
	origLookPath := ExecLookPath
	origDrop := DropPrivilegesFn
	origExec := ExecProgramFn
	t.Cleanup(func() {
		osReadlink = origReadlink
		osStat = origStat
		osGetuid = origGetuid
		osGeteuid = origGeteuid
		IsTerminal = origIsTerminal
		NewAuthenticator = origNewAuth
		PromptCredentials = origPrompt
		TuiPromptCredentials = origTuiPrompt
		ExecLookPath = origLookPath
		DropPrivilegesFn = origDrop
		ExecProgramFn = origExec
	})

	osReadlink = func(p string) (string, error) {
		if p == "/proc/self/fd/0" || p == "/proc/self/fd/1" || p == "/proc/self/fd/2" {
			return "/dev/tty1", nil
		}
		return "", os.ErrNotExist
	}
	osStat = func(p string) (os.FileInfo, error) {
		if p == "/dev/tty1" {
			return mockFileInfo{mode: os.ModeCharDevice}, nil
		}
		return nil, os.ErrNotExist
	}
	osGetuid = func() int { return 1000 }
	osGeteuid = func() int { return 1000 }
	IsTerminal = func(int) bool { return true }
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")

	NewAuthenticator = func(string) auth.Authenticator {
		t.Fatal("NewAuthenticator must not be called in post-login autostart mode")
		return nil
	}
	PromptCredentials = func(io.Reader, io.Writer) (string, string, error) {
		t.Fatal("PromptCredentials must not be called in post-login autostart mode")
		return "", "", nil
	}
	TuiPromptCredentials = func(string, string, string, func(string, string) error) (string, string, error) {
		t.Fatal("TuiPromptCredentials must not be called in post-login autostart mode")
		return "", "", nil
	}
	DropPrivilegesFn = func(string) error {
		t.Fatal("DropPrivilegesFn must not be called in post-login autostart mode")
		return nil
	}
	ExecLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }

	var execCalled bool
	ExecProgramFn = func(binary string, args []string, env []string) error {
		execCalled = true
		return nil
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	configPath := tmpDir + "/cdmrc"
	content := "binlist=(\"echo ok\")\nnamelist=(\"Ok\")\nflaglist=(\"C\")\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	exited := false
	Run([]string{"-config", configPath}, func(c int) {
		exited = true
		t.Fatalf("unexpected exit(%d)", c)
	})
	if exited {
		t.Fatal("did not expect exit")
	}
	if !execCalled {
		t.Fatal("expected user session command to execute without prompting for login")
	}
}

type countingAuthenticator struct {
	calls *int
}

func (a countingAuthenticator) Authenticate(username, password string) (auth.LoginSession, error) {
	*a.calls++
	return mockLoginSession{}, nil
}

func TestRunGettyAutoDetectAuthenticatesOnce(t *testing.T) {
	mockEnvFiles(t)
	mockUnsupported(t)

	origReadlink := osReadlink
	origStat := osStat
	origGetuid := osGetuid
	origGeteuid := osGeteuid
	origReadLoginUID := readLoginUID
	origIsTerminal := IsTerminal
	origNewAuth := NewAuthenticator
	origTuiPrompt := TuiPromptCredentials
	origLookPath := ExecLookPath
	origDrop := DropPrivilegesFn
	origExec := ExecProgramFn
	origChdir := ChdirFn
	origSaveState := SaveStateFn
	origPrepareTTY := PrepareTTYFn
	origRunSupervised := RunSupervisedSessionFn
	origSaveStateAsUser := SaveStateAsUserFn
	t.Cleanup(func() {
		osReadlink = origReadlink
		osStat = origStat
		osGetuid = origGetuid
		osGeteuid = origGeteuid
		readLoginUID = origReadLoginUID
		IsTerminal = origIsTerminal
		NewAuthenticator = origNewAuth
		TuiPromptCredentials = origTuiPrompt
		ExecLookPath = origLookPath
		DropPrivilegesFn = origDrop
		ExecProgramFn = origExec
		ChdirFn = origChdir
		SaveStateFn = origSaveState
		PrepareTTYFn = origPrepareTTY
		RunSupervisedSessionFn = origRunSupervised
		SaveStateAsUserFn = origSaveStateAsUser
	})
	ChdirFn = func(dir string) error { return nil }
	SaveStateFn = func(home, lastSession string, expectedUID int) error { return nil }
	PrepareTTYFn = func(acct *config.AccountInfo) (TTYCleanup, string, error) { return noopTTYCleanup, "", nil }
	SaveStateAsUserFn = func(acct *config.AccountInfo, lastSession string) error { return nil }

	for _, k := range []string{
		"DISPLAY", "WAYLAND_DISPLAY", "SSH_TTY", "SSH_CONNECTION", "SSH_CLIENT",
		"XDG_SESSION_ID", "GOCDM_SPAWN", "SHLVL", "USER", "LOGNAME", "SUDO_USER", "DOAS_USER",
	} {
		t.Setenv(k, "")
	}

	osReadlink = func(p string) (string, error) {
		if p == "/proc/self/fd/0" || p == "/proc/self/fd/1" {
			return "/dev/tty1", nil
		}
		return "", os.ErrNotExist
	}
	osStat = func(p string) (os.FileInfo, error) {
		if p == "/dev/tty1" {
			return mockFileInfo{mode: os.ModeCharDevice}, nil
		}
		return nil, os.ErrNotExist
	}
	osGetuid = func() int { return 0 }
	osGeteuid = func() int { return 0 }
	readLoginUID = func() (uint32, bool) { return unsetLoginUID, true }
	IsTerminal = func(int) bool { return true }

	authCalls := 0
	NewAuthenticator = func(string) auth.Authenticator {
		return countingAuthenticator{calls: &authCalls}
	}
	TuiPromptCredentials = func(title, theme, version string, authFunc func(string, string) error) (string, string, error) {
		err := authFunc("demo", "secret")
		return "demo", "secret", err
	}
	ExecLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }

	var droppedUser string
	var execCalled bool
	RunSupervisedSessionFn = func(spec SupervisedSessionSpec) (int, error) {
		if spec.Account != nil {
			droppedUser = spec.Account.Username
		}
		execCalled = true
		return 0, nil
	}

	tmpDir := t.TempDir()
	passwdPath := tmpDir + "/passwd"
	pamEnvPath := tmpDir + "/pam_env.conf"
	if err := os.WriteFile(passwdPath, []byte("demo:x:1001:1001:Demo:/home/demo:/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pamEnvPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	origPasswd := PasswdFilePath
	origPamEnv := PamEnvConfPath
	PasswdFilePath = passwdPath
	PamEnvConfPath = pamEnvPath
	t.Cleanup(func() {
		PasswdFilePath = origPasswd
		PamEnvConfPath = origPamEnv
	})

	configPath := tmpDir + "/cdmrc"
	content := "binlist=(\"echo ok\")\nnamelist=(\"Ok\")\nflaglist=(\"C\")\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	exited := false
	code := -1
	Run([]string{"-config", configPath}, func(c int) {
		exited = true
		code = c
	})

	if !exited || code != 0 {
		t.Fatalf("expected exit(0) after supervised session, got exited=%v code=%d", exited, code)
	}
	if authCalls != 1 {
		t.Fatalf("expected Authenticate to be called exactly once, got %d", authCalls)
	}
	if droppedUser != "demo" {
		t.Fatalf("expected supervised account for demo, got %q", droppedUser)
	}
	if !execCalled {
		t.Fatal("expected session program to be executed")
	}
}

func TestRunExplicitLoginFalseInGettyContextSkipsAuth(t *testing.T) {
	mockEnvFiles(t)
	mockUnsupported(t)

	origReadlink := osReadlink
	origStat := osStat
	origGetuid := osGetuid
	origGeteuid := osGeteuid
	origReadLoginUID := readLoginUID
	origIsTerminal := IsTerminal
	origNewAuth := NewAuthenticator
	origDrop := DropPrivilegesFn
	origLookPath := ExecLookPath
	origExec := ExecProgramFn
	t.Cleanup(func() {
		osReadlink = origReadlink
		osStat = origStat
		osGetuid = origGetuid
		osGeteuid = origGeteuid
		readLoginUID = origReadLoginUID
		IsTerminal = origIsTerminal
		NewAuthenticator = origNewAuth
		DropPrivilegesFn = origDrop
		ExecLookPath = origLookPath
		ExecProgramFn = origExec
	})

	for _, k := range []string{
		"DISPLAY", "WAYLAND_DISPLAY", "SSH_TTY", "SSH_CONNECTION", "SSH_CLIENT",
		"XDG_SESSION_ID", "GOCDM_SPAWN", "SHLVL", "USER", "LOGNAME", "SUDO_USER", "DOAS_USER",
	} {
		t.Setenv(k, "")
	}

	osReadlink = func(p string) (string, error) { return "/dev/tty1", nil }
	osStat = func(p string) (os.FileInfo, error) { return mockFileInfo{mode: os.ModeCharDevice}, nil }
	osGetuid = func() int { return 0 }
	osGeteuid = func() int { return 0 }
	readLoginUID = func() (uint32, bool) { return unsetLoginUID, true }
	IsTerminal = func(int) bool { return true }

	NewAuthenticator = func(string) auth.Authenticator {
		t.Fatal("NewAuthenticator must not be called with -login=false")
		return nil
	}
	DropPrivilegesFn = func(string) error {
		t.Fatal("DropPrivilegesFn must not be called with -login=false")
		return nil
	}
	ExecLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	execCalled := false
	ExecProgramFn = func(string, []string, []string) error {
		execCalled = true
		return nil
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	configPath := tmpDir + "/cdmrc"
	if err := os.WriteFile(configPath, []byte("binlist=(\"echo ok\")\nnamelist=(\"Ok\")\nflaglist=(\"C\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	Run([]string{"-config", configPath, "-login=false"}, func(c int) {
		t.Fatalf("unexpected exit(%d)", c)
	})
	if !execCalled {
		t.Fatal("expected ExecProgramFn to be called")
	}
}

func TestRunNonRootExplicitLoginFailsBeforeAuth(t *testing.T) {
	mockUnsupported(t)

	origGetuid := osGetuid
	origGeteuid := osGeteuid
	origNewAuth := NewAuthenticator
	origPrompt := PromptCredentials
	origTuiPrompt := TuiPromptCredentials
	t.Cleanup(func() {
		osGetuid = origGetuid
		osGeteuid = origGeteuid
		NewAuthenticator = origNewAuth
		PromptCredentials = origPrompt
		TuiPromptCredentials = origTuiPrompt
	})

	osGetuid = func() int { return 1000 }
	osGeteuid = func() int { return 1000 }
	NewAuthenticator = func(string) auth.Authenticator {
		t.Fatal("NewAuthenticator must not be called when non-root -login is rejected")
		return nil
	}
	PromptCredentials = func(io.Reader, io.Writer) (string, string, error) {
		t.Fatal("PromptCredentials must not be called when non-root -login is rejected")
		return "", "", nil
	}
	TuiPromptCredentials = func(string, string, string, func(string, string) error) (string, string, error) {
		t.Fatal("TuiPromptCredentials must not be called when non-root -login is rejected")
		return "", "", nil
	}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	exited := false
	code := -1
	Run([]string{"-login"}, func(c int) { exited = true; code = c })
	_ = w.Close()
	out, _ := io.ReadAll(r)

	if !exited || code != 1 {
		t.Fatalf("expected exit(1), got exited=%v code=%d", exited, code)
	}
	if !strings.Contains(string(out), "requires root privileges") {
		t.Fatalf("expected root privileges diagnostic on stderr, got %q", string(out))
	}
}

func TestRunModeFlagErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "conflicting login and mode user",
			args:    []string{"-login", "-mode=user"},
			wantErr: "conflicting mode flags",
		},
		{
			name:    "invalid mode value",
			args:    []string{"-mode=bogus"},
			wantErr: fmt.Sprintf("invalid -mode %q", "bogus"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w
			defer func() { os.Stderr = oldStderr }()

			exited := false
			code := -1
			Run(tc.args, func(c int) { exited = true; code = c })
			_ = w.Close()
			out, _ := io.ReadAll(r)

			if !exited || code != 2 {
				t.Fatalf("expected exit(2), got exited=%v code=%d", exited, code)
			}
			if !strings.Contains(string(out), tc.wantErr) {
				t.Fatalf("expected stderr containing %q, got %q", tc.wantErr, string(out))
			}
		})
	}
}
