package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeReadUserFile(t *testing.T) {
	home := t.TempDir()
	uid := os.Geteuid()

	if err := os.MkdirAll(filepath.Join(home, ".config", "cdm"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".config", "cdm", "cdmrc")
	if err := os.WriteFile(target, []byte("countfrom=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("valid nested file read", func(t *testing.T) {
		data, err := SafeReadUserFile(home, filepath.Join(".config", "cdm", "cdmrc"), uid, false)
		if err != nil {
			t.Fatalf("SafeReadUserFile failed: %v", err)
		}
		if string(data) != "countfrom=2\n" {
			t.Fatalf("unexpected data %q", string(data))
		}
	})

	t.Run("missing file returns ErrNotExist", func(t *testing.T) {
		_, err := SafeReadUserFile(home, ".cdmrc", uid, false)
		if err == nil || !os.IsNotExist(err) {
			t.Fatalf("expected IsNotExist error, got %v", err)
		}
	})

	t.Run("relative traversal rejected", func(t *testing.T) {
		if _, err := SafeReadUserFile(home, "../etc/passwd", uid, false); err == nil {
			t.Fatal("expected traversal error")
		}
		if _, err := SafeReadUserFile(home, "/etc/passwd", uid, false); err == nil {
			t.Fatal("expected absolute path error")
		}
	})

	t.Run("symlink file rejected", func(t *testing.T) {
		secret := filepath.Join(t.TempDir(), "secret")
		if err := os.WriteFile(secret, []byte("topsecret"), 0o600); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(home, ".cdmrc")
		if err := os.Symlink(secret, linkPath); err != nil {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		_, err := SafeReadUserFile(home, ".cdmrc", uid, false)
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("expected symlink refusal, got %v", err)
		}
	})

	t.Run("symlink intermediate dir rejected", func(t *testing.T) {
		otherDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(otherDir, "state.json"), []byte(`{"last_session":"x"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		home2 := t.TempDir()
		if err := os.Symlink(otherDir, filepath.Join(home2, ".config")); err != nil {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		_, err := SafeReadUserFile(home2, filepath.Join(".config", "state.json"), uid, false)
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("expected symlink dir refusal, got %v", err)
		}
	})
}

func TestSafeSaveStateAt(t *testing.T) {
	t.Run("atomic write and reload", func(t *testing.T) {
		home := t.TempDir()
		uid := os.Geteuid()
		if err := SafeSaveStateAt(home, "zsh", uid); err != nil {
			t.Fatalf("SafeSaveStateAt failed: %v", err)
		}
		state, err := LoadStateForUser(home, uid)
		if err != nil {
			t.Fatalf("LoadStateForUser failed: %v", err)
		}
		if state.LastSession != "zsh" {
			t.Fatalf("got LastSession %q, want zsh", state.LastSession)
		}
	})

	t.Run("absent home directory is not created", func(t *testing.T) {
		missingHome := filepath.Join(t.TempDir(), "nonexistent-user-home")
		err := SafeSaveStateAt(missingHome, "bash", os.Geteuid())
		if err == nil {
			t.Fatal("expected error when home directory does not exist")
		}
		if _, statErr := os.Stat(missingHome); !os.IsNotExist(statErr) {
			t.Fatalf("SafeSaveStateAt must not create missing home directory, statErr=%v", statErr)
		}
	})

	t.Run("refuses writing non-root user state as root", func(t *testing.T) {
		origGeteuid := stateGeteuid
		stateGeteuid = func() int { return 0 }
		t.Cleanup(func() { stateGeteuid = origGeteuid })

		home := t.TempDir()
		err := SafeSaveStateAt(home, "bash", 1001)
		if err == nil || !strings.Contains(err.Error(), "running as root") {
			t.Fatalf("expected root write refusal, got %v", err)
		}
	})

	t.Run("symlink .config rejected", func(t *testing.T) {
		home := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(home, ".config")); err != nil {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		err := SafeSaveStateAt(home, "bash", os.Geteuid())
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("expected symlink refusal, got %v", err)
		}
	})

	t.Run("symlink .config/gocdm rejected", func(t *testing.T) {
		home := t.TempDir()
		outside := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(home, ".config", "gocdm")); err != nil {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		err := SafeSaveStateAt(home, "bash", os.Geteuid())
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("expected symlink refusal, got %v", err)
		}
	})

	t.Run("symlink state.json rejected", func(t *testing.T) {
		home := t.TempDir()
		target := filepath.Join(t.TempDir(), "victim")
		if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
		stateDir := filepath.Join(home, ".config", "gocdm")
		if err := os.MkdirAll(stateDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(stateDir, "state.json")); err != nil {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		err := SafeSaveStateAt(home, "bash", os.Geteuid())
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("expected symlink refusal, got %v", err)
		}
		data, _ := os.ReadFile(target)
		if string(data) != "original" {
			t.Fatalf("victim file was modified: %q", string(data))
		}
	})

	t.Run("wrong-uid existing state diagnosed without mutating file", func(t *testing.T) {
		home := t.TempDir()
		stateDir := filepath.Join(home, ".config", "gocdm")
		if err := os.MkdirAll(stateDir, 0o700); err != nil {
			t.Fatal(err)
		}
		statePath := filepath.Join(stateDir, "state.json")
		if err := os.WriteFile(statePath, []byte(`{"last_session":"legacy"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		otherUID := os.Geteuid() + 12345
		err := SafeSaveStateAt(home, "newsession", otherUID)
		if err == nil || !strings.Contains(err.Error(), "owned by uid") {
			t.Fatalf("expected ownership diagnostic error, got %v", err)
		}
		data, _ := os.ReadFile(statePath)
		if string(data) != `{"last_session":"legacy"}` {
			t.Fatalf("existing state file must not be overwritten, got %q", string(data))
		}
	})
}

func TestLoadSystemConfigAndUserConfig(t *testing.T) {
	tmpDir := t.TempDir()
	sysCfg := filepath.Join(tmpDir, "etc-cdmrc")
	rootHome := filepath.Join(tmpDir, "root-home")
	aliceHome := filepath.Join(tmpDir, "alice-home")
	if err := os.MkdirAll(rootHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(aliceHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sysCfg, []byte("countfrom=5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootHome, ".cdmrc"), []byte("countfrom=99\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aliceHome, ".cdmrc"), []byte("countfrom=7\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origSys := SystemConfigPath
	SystemConfigPath = sysCfg
	t.Cleanup(func() { SystemConfigPath = origSys })

	t.Setenv("HOME", rootHome)

	sysLoaded, err := LoadSystemConfig("")
	if err != nil {
		t.Fatalf("LoadSystemConfig failed: %v", err)
	}
	if sysLoaded.CountFrom != 5 {
		t.Fatalf("LoadSystemConfig must ignore launcher HOME, got countfrom=%d (want 5)", sysLoaded.CountFrom)
	}

	aliceLoaded, err := LoadConfigForUser("", aliceHome, os.Geteuid(), true)
	if err != nil {
		t.Fatalf("LoadConfigForUser failed: %v", err)
	}
	if aliceLoaded.CountFrom != 7 {
		t.Fatalf("LoadConfigForUser got countfrom=%d, want 7", aliceLoaded.CountFrom)
	}

	bobHome := filepath.Join(tmpDir, "bob-home")
	if err := os.MkdirAll(bobHome, 0o755); err != nil {
		t.Fatal(err)
	}
	bobLoaded, err := LoadConfigForUser("", bobHome, os.Geteuid(), true)
	if err != nil {
		t.Fatalf("LoadConfigForUser fallback failed: %v", err)
	}
	if bobLoaded.CountFrom != 5 {
		t.Fatalf("LoadConfigForUser fallback got countfrom=%d, want 5", bobLoaded.CountFrom)
	}
}
