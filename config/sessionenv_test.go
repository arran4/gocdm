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
