package session

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed testdata/edgecases/*.desktop
var edgecaseDesktopFiles embed.FS

func writeEmbeddedDesktop(t *testing.T, baseDir, name string) string {
	t.Helper()
	content, err := edgecaseDesktopFiles.ReadFile("testdata/edgecases/" + name)
	if err != nil {
		t.Fatalf("read embedded desktop %s: %v", name, err)
	}
	path := filepath.Join(baseDir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write desktop %s: %v", name, err)
	}
	return path
}

func withTempPathExecutable(t *testing.T, bin string) {
	t.Helper()
	origLookPath := ExecLookPath
	ExecLookPath = func(file string) (string, error) {
		return file, nil
	}
	t.Cleanup(func() { ExecLookPath = origLookPath })
}

func TestParseDesktopFileWhitespaceAroundEquals(t *testing.T) {
	withTempPathExecutable(t, "edge-session")
	tmpDir, err := os.MkdirTemp("", "desktop-whitespace")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	path := writeEmbeddedDesktop(t, tmpDir, "whitespace.desktop")

	s, err := parseDesktopFile(path, "X")
	if err != nil {
		t.Fatalf("parseDesktopFile failed: %v", err)
	}
	if s.Name != "Whitespace Session" {
		t.Fatalf("expected parsed name, got %q", s.Name)
	}
	if s.Exec != "edge-session --flag" {
		t.Fatalf("expected Exec freedesktop token stripped, got %q", s.Exec)
	}
}

func TestParseDesktopFileLocalizedNameFallback(t *testing.T) {
	withTempPathExecutable(t, "edge-session")
	tmpDir, err := os.MkdirTemp("", "desktop-localized")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	path := writeEmbeddedDesktop(t, tmpDir, "localized_name_only.desktop")

	s, err := parseDesktopFile(path, "W")
	if err != nil {
		t.Fatalf("parseDesktopFile failed: %v", err)
	}
	if s.Name != "Localized Session" {
		t.Fatalf("expected localized fallback name, got %q", s.Name)
	}
	if s.Type != "W" {
		t.Fatalf("expected type W, got %q", s.Type)
	}
}

func TestParseDesktopFileQuotedExecPreserved(t *testing.T) {
	withTempPathExecutable(t, "/bin/sh")
	tmpDir, err := os.MkdirTemp("", "desktop-quoted")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	path := writeEmbeddedDesktop(t, tmpDir, "quoted_exec.desktop")

	s, err := parseDesktopFile(path, "X")
	if err != nil {
		t.Fatalf("parseDesktopFile failed: %v", err)
	}
	if !strings.Contains(s.Exec, "-lc \"printf") {
		t.Fatalf("expected quoted shell fragment preserved, got %q", s.Exec)
	}
	if strings.Contains(s.Exec, "%u") {
		t.Fatalf("expected freedesktop token stripped from Exec, got %q", s.Exec)
	}
}
