package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSessionEnv(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sessionenv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	passwdPath := filepath.Join(tmpDir, "passwd")
	if err := os.WriteFile(passwdPath, []byte("demo:x:1000:1000:Demo:/home/demo:/bin/bash\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pamEnvPath := filepath.Join(tmpDir, "pam_env.conf")
	pamEnv := "XDG_SESSION_TYPE DEFAULT=tty\nEDITOR DEFAULT=vim\nGREETING OVERRIDE=hello_$USER\n"
	if err := os.WriteFile(pamEnvPath, []byte(pamEnv), 0644); err != nil {
		t.Fatal(err)
	}

	env, err := BuildSessionEnv([]string{"EDITOR=nano"}, "demo", passwdPath, pamEnvPath)
	if err != nil {
		t.Fatalf("BuildSessionEnv failed: %v", err)
	}

	envMap := EnvSliceToMap(env)
	if got := envMap["USER"]; got != "demo" {
		t.Fatalf("USER mismatch: got %q", got)
	}
	if got := envMap["HOME"]; got != "/home/demo" {
		t.Fatalf("HOME mismatch: got %q", got)
	}
	if got := envMap["SHELL"]; got != "/bin/bash" {
		t.Fatalf("SHELL mismatch: got %q", got)
	}
	if got := envMap["XDG_SESSION_TYPE"]; got != "tty" {
		t.Fatalf("XDG_SESSION_TYPE mismatch: got %q", got)
	}
	if got := envMap["EDITOR"]; got != "nano" {
		t.Fatalf("EDITOR should keep existing value, got %q", got)
	}
	if got := envMap["GREETING"]; got != "hello_demo" {
		t.Fatalf("GREETING mismatch: got %q", got)
	}
}

func TestBuildSessionEnvMissingUser(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sessionenv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	passwdPath := filepath.Join(tmpDir, "passwd")
	if err := os.WriteFile(passwdPath, []byte("demo:x:1000:1000:Demo:/home/demo:/bin/bash\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = BuildSessionEnv(nil, "missing", passwdPath, "")
	if err == nil {
		t.Fatal("expected error for missing user")
	}
}

func TestParsePasswdLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected *PasswdEntry
	}{
		{
			name: "Valid entry",
			line: "demo:x:1000:1000:Demo:/home/demo:/bin/bash",
			expected: &PasswdEntry{
				Username: "demo",
				HomeDir:  "/home/demo",
				Shell:    "/bin/bash",
			},
		},
		{
			name:     "Invalid entry missing fields",
			line:     "demo:x:1000:1000",
			expected: nil,
		},
		{
			name: "Empty shell",
			line: "nobody:x:65534:65534:nobody:/nonexistent:",
			expected: &PasswdEntry{
				Username: "nobody",
				HomeDir:  "/nonexistent",
				Shell:    "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePasswdLine(tt.line)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %+v, got nil", tt.expected)
			}
			if got.Username != tt.expected.Username || got.HomeDir != tt.expected.HomeDir || got.Shell != tt.expected.Shell {
				t.Errorf("expected %+v, got %+v", tt.expected, got)
			}
		})
	}
}

func TestLoadPasswdEntryEdgeCases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sessionenv_passwd")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	passwdData := `
# Comment line

malformed:line:with:few:fields
demouser:x:1000:1000:Demo User:/home/demouser:/bin/zsh
demouser2:x:1001:1001:Demo User 2:/home/demouser2:/bin/bash
`
	passwdPath := filepath.Join(tmpDir, "passwd")
	if err := os.WriteFile(passwdPath, []byte(passwdData), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		username  string
		expectErr bool
		expected  *PasswdEntry
	}{
		{
			name:      "Valid user",
			username:  "demouser",
			expectErr: false,
			expected: &PasswdEntry{
				Username: "demouser",
				HomeDir:  "/home/demouser",
				Shell:    "/bin/zsh",
			},
		},
		{
			name:      "Valid user 2",
			username:  "demouser2",
			expectErr: false,
			expected: &PasswdEntry{
				Username: "demouser2",
				HomeDir:  "/home/demouser2",
				Shell:    "/bin/bash",
			},
		},
		{
			name:      "User not found",
			username:  "missinguser",
			expectErr: true,
		},
		{
			name:      "Malformed user matched prefix but invalid",
			username:  "malformed",
			expectErr: true, // Should continue searching and eventually fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadPasswdEntry(passwdPath, tt.username)
			if (err != nil) != tt.expectErr {
				t.Errorf("expected error: %v, got: %v", tt.expectErr, err)
			}
			if !tt.expectErr && tt.expected != nil {
				if got == nil {
					t.Fatalf("expected %+v, got nil", tt.expected)
				}
				if got.Username != tt.expected.Username || got.HomeDir != tt.expected.HomeDir || got.Shell != tt.expected.Shell {
					t.Errorf("expected %+v, got %+v", tt.expected, got)
				}
			}
		})
	}
}

func TestBuildGettySessionEnvTrustBoundary(t *testing.T) {
	tmpDir := t.TempDir()
	pamEnvPath := filepath.Join(tmpDir, "pam_env.conf")
	pamEnvData := "EDITOR DEFAULT=vim\nLANG DEFAULT=C.UTF-8\nGREETING OVERRIDE=hi_$USER\n"
	if err := os.WriteFile(pamEnvPath, []byte(pamEnvData), 0o644); err != nil {
		t.Fatal(err)
	}

	acct := &AccountInfo{
		Username: "alice",
		UID:      1001,
		GID:      1001,
		HomeDir:  "/home/alice",
		Shell:    "/bin/zsh",
	}

	launcherEnv := []string{
		"USER=root",
		"LOGNAME=root",
		"HOME=/root",
		"SHELL=/bin/sh",
		"PATH=/root/evil:/usr/sbin",
		"AWS_SECRET_ACCESS_KEY=supersecret",
		"LD_PRELOAD=/tmp/evil.so",
		"INVOCATION_ID=systemd-getty-123",
		"JOURNAL_STREAM=8:1234",
		"XDG_RUNTIME_DIR=/run/user/0",
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/0/bus",
		"TERM=xterm-256color",
		"LANG=en_AU.UTF-8",
		"LC_TIME=en_GB.UTF-8",
	}

	t.Run("strips secrets and preserves PAM and locale", func(t *testing.T) {
		env, err := BuildGettySessionEnv(GettyEnvOptions{
			LauncherEnv: launcherEnv,
			Account:     acct,
			PamEnvFile:  pamEnvPath,
			PamSessionEnv: []string{
				"XDG_RUNTIME_DIR=/run/user/1001",
				"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus",
				"XDG_SESSION_ID=7",
			},
			SessionType: "W",
			SessionName: "Plasma (Wayland)",
			WorkingDir:  "/home/alice",
			SpawnPID:    4242,
		})
		if err != nil {
			t.Fatalf("BuildGettySessionEnv failed: %v", err)
		}

		envMap := EnvSliceToMap(env)
		for _, leaked := range []string{"AWS_SECRET_ACCESS_KEY", "LD_PRELOAD", "INVOCATION_ID", "JOURNAL_STREAM"} {
			if _, ok := envMap[leaked]; ok {
				t.Fatalf("launcher variable %s must not leak into session env", leaked)
			}
		}
		if envMap["USER"] != "alice" || envMap["LOGNAME"] != "alice" || envMap["HOME"] != "/home/alice" || envMap["SHELL"] != "/bin/zsh" {
			t.Fatalf("unexpected identity vars: USER=%q LOGNAME=%q HOME=%q SHELL=%q", envMap["USER"], envMap["LOGNAME"], envMap["HOME"], envMap["SHELL"])
		}
		if envMap["PATH"] != "/usr/local/bin:/usr/bin:/bin" {
			t.Fatalf("unexpected non-root PATH: %q", envMap["PATH"])
		}
		if envMap["TERM"] != "xterm-256color" || envMap["LANG"] != "en_AU.UTF-8" || envMap["LC_TIME"] != "en_GB.UTF-8" {
			t.Fatalf("expected allowlisted TERM/locale preserved, got TERM=%q LANG=%q LC_TIME=%q", envMap["TERM"], envMap["LANG"], envMap["LC_TIME"])
		}
		if envMap["EDITOR"] != "vim" || envMap["GREETING"] != "hi_alice" {
			t.Fatalf("expected pam_env.conf applied, got EDITOR=%q GREETING=%q", envMap["EDITOR"], envMap["GREETING"])
		}
		if envMap["XDG_RUNTIME_DIR"] != "/run/user/1001" || envMap["DBUS_SESSION_BUS_ADDRESS"] != "unix:path=/run/user/1001/bus" || envMap["XDG_SESSION_ID"] != "7" {
			t.Fatalf("expected authoritative PAM env values, got %v", envMap)
		}
		if envMap["XDG_SESSION_TYPE"] != "wayland" || envMap["XDG_SESSION_CLASS"] != "user" || envMap["DESKTOP_SESSION"] != "Plasma (Wayland)" {
			t.Fatalf("unexpected wayland session vars: %v", envMap)
		}
		if envMap["PWD"] != "/home/alice" || envMap["GOCDM_SPAWN"] != "4242" {
			t.Fatalf("unexpected PWD/GOCDM_SPAWN: PWD=%q GOCDM_SPAWN=%q", envMap["PWD"], envMap["GOCDM_SPAWN"])
		}
	})

	t.Run("does not fabricate runtime or dbus vars and omits desktop vars for console", func(t *testing.T) {
		env, err := BuildGettySessionEnv(GettyEnvOptions{
			LauncherEnv: launcherEnv,
			Account:     acct,
			SessionType: "C",
			SessionName: "zsh",
			WorkingDir:  "/home/alice",
		})
		if err != nil {
			t.Fatalf("BuildGettySessionEnv failed: %v", err)
		}
		envMap := EnvSliceToMap(env)
		for _, forbidden := range []string{"XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS", "XDG_SESSION_ID", "DESKTOP_SESSION", "XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP"} {
			if _, ok := envMap[forbidden]; ok {
				t.Fatalf("console env without PAM session vars must not fabricate %s (got %q)", forbidden, envMap[forbidden])
			}
		}
		if envMap["XDG_SESSION_TYPE"] != "tty" || envMap["XDG_SESSION_CLASS"] != "user" {
			t.Fatalf("unexpected console XDG_SESSION vars: %v", envMap)
		}
	})

	t.Run("root account gets sbin PATH and default TERM linux", func(t *testing.T) {
		env, err := BuildGettySessionEnv(GettyEnvOptions{
			Account:     &AccountInfo{Username: "root", UID: 0, HomeDir: "/root", Shell: "/bin/bash"},
			SessionType: "C",
		})
		if err != nil {
			t.Fatalf("BuildGettySessionEnv failed: %v", err)
		}
		envMap := EnvSliceToMap(env)
		if envMap["PATH"] != "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" {
			t.Fatalf("unexpected root PATH: %q", envMap["PATH"])
		}
		if envMap["TERM"] != "linux" {
			t.Fatalf("expected default TERM=linux, got %q", envMap["TERM"])
		}
	})

	t.Run("errors on nil account or unreadable pam_env path", func(t *testing.T) {
		if _, err := BuildGettySessionEnv(GettyEnvOptions{}); err == nil {
			t.Fatal("expected error for nil Account")
		}
		if _, err := BuildGettySessionEnv(GettyEnvOptions{
			Account:    acct,
			PamEnvFile: tmpDir, // directory cannot be scanned as file
		}); err == nil {
			t.Fatal("expected error when reading unreadable pam_env path")
		}
	})
}

func TestBuildUserSessionEnv(t *testing.T) {
	env := BuildUserSessionEnv(UserEnvOptions{
		BaseEnv:     []string{"USER=alice", "CUSTOM_VAR=kept"},
		SessionType: "X",
		SessionName: "i3",
		SpawnPID:    99,
	})
	envMap := EnvSliceToMap(env)
	if envMap["CUSTOM_VAR"] != "kept" {
		t.Fatalf("expected inherited user variable kept, got %q", envMap["CUSTOM_VAR"])
	}
	if _, ok := envMap["XDG_RUNTIME_DIR"]; ok {
		t.Fatal("must not fabricate XDG_RUNTIME_DIR")
	}
	if envMap["XDG_SESSION_TYPE"] != "x11" || envMap["DESKTOP_SESSION"] != "i3" || envMap["GOCDM_SPAWN"] != "99" {
		t.Fatalf("unexpected session metadata: %v", envMap)
	}
}
