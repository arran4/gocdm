package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateHomeDir(t *testing.T) {
	tests := []struct {
		name    string
		homeDir string
		wantErr bool
	}{
		{"valid absolute path", "/home/alice", false},
		{"valid root", "/", false},
		{"empty path", "", true},
		{"relative path", "home/alice", true},
		{"traversal segment", "/home/../etc", true},
		{"trailing traversal", "/home/alice/..", true},
		{"nul byte", "/home/alice\x00evil", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateHomeDir(tc.homeDir)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateHomeDir(%q) error = %v, wantErr=%v", tc.homeDir, err, tc.wantErr)
			}
		})
	}
}

func TestLookupAccount(t *testing.T) {
	tmpDir := t.TempDir()
	passwdPath := filepath.Join(tmpDir, "passwd")
	passwdContent := strings.Join([]string{
		"root:x:0:0:root:/root:/bin/bash",
		"alice:x:1001:1002:Alice:/home/alice:/bin/zsh",
		"noshell:x:1003:1003:NoShell:/home/noshell:",
		"baduid:x:notanumber:1004:Bad:/home/bad:/bin/sh",
	}, "\n") + "\n"
	if err := os.WriteFile(passwdPath, []byte(passwdContent), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("valid user", func(t *testing.T) {
		acct, err := LookupAccount("alice", passwdPath)
		if err != nil {
			t.Fatalf("LookupAccount failed: %v", err)
		}
		if acct.Username != "alice" || acct.UID != 1001 || acct.GID != 1002 || acct.HomeDir != "/home/alice" || acct.Shell != "/bin/zsh" {
			t.Fatalf("unexpected account info: %+v", acct)
		}
		if len(acct.Groups) != 1 || acct.Groups[0] != 1002 {
			t.Fatalf("expected primary group [1002], got %v", acct.Groups)
		}
	})

	t.Run("empty shell defaults to /bin/sh", func(t *testing.T) {
		acct, err := LookupAccount("noshell", passwdPath)
		if err != nil {
			t.Fatalf("LookupAccount failed: %v", err)
		}
		if acct.Shell != "/bin/sh" {
			t.Fatalf("expected /bin/sh fallback, got %q", acct.Shell)
		}
	})

	t.Run("missing user fails", func(t *testing.T) {
		if _, err := LookupAccount("missing", passwdPath); err == nil {
			t.Fatal("expected error for missing user")
		}
	})

	t.Run("bad uid entry fails", func(t *testing.T) {
		if _, err := LookupAccount("baduid", passwdPath); err == nil {
			t.Fatal("expected error for non-numeric uid")
		}
	})

	t.Run("invalid username characters rejected", func(t *testing.T) {
		for _, bad := range []string{"", "alice:0", "../root", "alice bob", "alice\x00"} {
			if _, err := LookupAccount(bad, passwdPath); err == nil {
				t.Fatalf("expected error for username %q", bad)
			}
		}
	})
}
