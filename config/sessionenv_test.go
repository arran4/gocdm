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

	envMap := envSliceToMap(env)
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
		expected *passwdEntry
	}{
		{
			name: "Valid entry",
			line: "demo:x:1000:1000:Demo:/home/demo:/bin/bash",
			expected: &passwdEntry{
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
			expected: &passwdEntry{
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
		expected  *passwdEntry
	}{
		{
			name:      "Valid user",
			username:  "demouser",
			expectErr: false,
			expected: &passwdEntry{
				Username: "demouser",
				HomeDir:  "/home/demouser",
				Shell:    "/bin/zsh",
			},
		},
		{
			name:      "Valid user 2",
			username:  "demouser2",
			expectErr: false,
			expected: &passwdEntry{
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
			got, err := loadPasswdEntry(passwdPath, tt.username)
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
