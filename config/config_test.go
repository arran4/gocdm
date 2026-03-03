package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	content := `
dialogrc="/tmp/dialogrc"
countfrom=1
display=1
xtty="8"
locktty=yes
consolekit=no
cktimeout=60
altstartx=yes
startxlog="/tmp/log"
binlist=("a" "b")
namelist=("A" "B")
flaglist=("X" "C")
serverargs=("-a" "-b")
`
	tmpfile, err := os.CreateTemp("", "cdmrc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	expected := &Config{
		DialogRC:   "/tmp/dialogrc",
		CountFrom:  1,
		Display:    1,
		XTTY:       "8",
		LockTTY:    true,
		ConsoleKit: false,
		CKTimeout:  60,
		AltStartX:  true,
		StartXLog:  "/tmp/log",
		BinList:    []string{"a", "b"},
		NameList:   []string{"A", "B"},
		FlagList:   []string{"X", "C"},
		ServerArgs: []string{"-a", "-b"},
	}

	if !reflect.DeepEqual(cfg, expected) {
		t.Errorf("Expected %+v, got %+v", expected, cfg)
	}
}

func TestLoadConfigWithSpace(t *testing.T) {
	content := `
countfrom=1
`
	tmpDir, err := os.MkdirTemp("", "cdm test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tmpfile := filepath.Join(tmpDir, "config with space")
	if err := os.WriteFile(tmpfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpfile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.CountFrom != 1 {
		t.Errorf("Expected CountFrom 1, got %d", cfg.CountFrom)
	}
}

func TestLoadConfigMalformedArray(t *testing.T) {
	content := `binlist="a" "b"`
	p := filepath.Join(t.TempDir(), "cdmrc")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(p)
	if err == nil || !strings.Contains(err.Error(), "malformed array") {
		t.Fatalf("expected malformed array error, got %v", err)
	}
}

func TestLoadConfigMismatchedLists(t *testing.T) {
	content := `
binlist=("a" "b")
namelist=("A")
`
	p := filepath.Join(t.TempDir(), "cdmrc")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(p)
	if err == nil || !strings.Contains(err.Error(), "namelist length") {
		t.Fatalf("expected namelist mismatch error, got %v", err)
	}
}

func TestLoadConfigDefaultDiscoveryOrder(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, ".config")
	if err := os.MkdirAll(filepath.Join(xdg, "cdm"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	xdgPath := filepath.Join(xdg, "cdm", "cdmrc")
	homePath := filepath.Join(home, ".cdmrc")
	if err := os.WriteFile(xdgPath, []byte("countfrom=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(homePath, []byte("countfrom=3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CountFrom != 3 {
		t.Fatalf("expected HOME .cdmrc precedence, got countfrom=%d", cfg.CountFrom)
	}
}
