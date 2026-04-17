package session

import (
	_ "embed"
	"os"
	"path/filepath"
	"testing"
)

//go:embed testdata/missing_exec.desktop
var missingExecContent []byte

//go:embed testdata/missing_tryexec.desktop
var missingTryExecContent []byte

//go:embed testdata/missing_exec_bin.desktop
var missingExecBinContent []byte

func TestDiscoverSessions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	testCommand := setupPathExecutable(t, tmpDir)

	// Directory structure
	x11Dir := filepath.Join(tmpDir, "etc", "X11", "Sessions")
	xsessionsDir := filepath.Join(tmpDir, "usr", "share", "xsessions")
	waylandSessionsDir := filepath.Join(tmpDir, "usr", "share", "wayland-sessions")
	userHome := filepath.Join(tmpDir, "home", "user")
	userConfigWayland := filepath.Join(userHome, ".config", "wayland-sessions")

	dirs := []string{x11Dir, xsessionsDir, waylandSessionsDir, userHome, userConfigWayland}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Legacy X11 Session
	if err := os.WriteFile(filepath.Join(x11Dir, "legacy_x11"), []byte("#!/bin/sh\necho legacy"), 0755); err != nil {
		t.Fatal(err)
	}

	// 2. Standard XSession .desktop
	desktopContent := "[Desktop Entry]\n" +
		"Name=Standard X Session\n" +
		"Exec=" + testCommand + "\n"
	if err := os.WriteFile(filepath.Join(xsessionsDir, "standard.desktop"), []byte(desktopContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Wayland Session .desktop
	waylandContent := "[Desktop Entry]\n" +
		"Name=Wayland Session\n" +
		"Exec=" + testCommand + "\n"
	if err := os.WriteFile(filepath.Join(waylandSessionsDir, "wayland.desktop"), []byte(waylandContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. User .xinitrc
	if err := os.WriteFile(filepath.Join(userHome, ".xinitrc"), []byte("#!/bin/sh\necho user"), 0755); err != nil {
		t.Fatal(err)
	}

	// 5. User Custom Wayland Session
	userWaylandContent := "[Desktop Entry]\n" +
		"Name=User Wayland\n" +
		"Exec=" + testCommand + "\n"
	if err := os.WriteFile(filepath.Join(userConfigWayland, "user-wayland.desktop"), []byte(userWaylandContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 6. Shells file
	shellsFilePath := filepath.Join(tmpDir, "shells")
	shellsContent := "# /etc/shells: valid login shells\n" +
		testCommand + "\n"
	if err := os.WriteFile(shellsFilePath, []byte(shellsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Save original vars
	origX11 := X11SessionsDir
	origXSessions := XSessionsDir
	origWaylandSessions := WaylandSessionsDir
	origShellsFile := ShellsFile
	defer func() {
		X11SessionsDir = origX11
		XSessionsDir = origXSessions
		WaylandSessionsDir = origWaylandSessions
		ShellsFile = origShellsFile
	}()

	X11SessionsDir = x11Dir
	XSessionsDir = xsessionsDir
	WaylandSessionsDir = waylandSessionsDir
	ShellsFile = shellsFilePath

	// Test Discovery
	sessions, err := DiscoverSessions(userHome)
	if err != nil {
		t.Fatalf("DiscoverSessions failed: %v", err)
	}

	// Expected sessions:
	// 1. Custom X Session (.xinitrc) (Type X)
	// 2. User Wayland (Type W)
	// 3. legacy_x11 (Type X)
	// 4. Standard X Session (Type X)
	// 5. Wayland Session (Type W)
	// 6. test-executable (Type C) (from shells file)

	expectedCount := 6
	if len(sessions) != expectedCount {
		t.Errorf("Expected %d sessions, got %d", expectedCount, len(sessions))
		for _, s := range sessions {
			t.Logf("Found: %s (%s)", s.Name, s.Type)
		}
	}

	// Verify specific sessions presence
	found := make(map[string]string) // Name -> Type
	for _, s := range sessions {
		found[s.Name] = s.Type
	}

	expectations := map[string]string{
		"Custom X Session (.xinitrc)": "X",
		"User Wayland":                "W",
		"legacy_x11":                  "X",
		"Standard X Session":          "X",
		"Wayland Session":             "W",
		filepath.Base(testCommand):    "C",
	}

	for name, typ := range expectations {
		if gotType, ok := found[name]; !ok {
			t.Errorf("Expected session '%s' not found", name)
		} else if gotType != typ {
			t.Errorf("Expected session '%s' to have type '%s', got '%s'", name, typ, gotType)
		}
	}
}

func TestDiscoverShellSessions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	testCommand := setupPathExecutable(t, tmpDir)

	shellsFilePath := filepath.Join(tmpDir, "shells")
	shellsContent := "# /etc/shells: valid login shells\n" +
		"/bin/false\n" + // Assuming it's missing or we mock it via missing exec
		testCommand + "\n" +
		"   \n" // empty line
	if err := os.WriteFile(shellsFilePath, []byte(shellsContent), 0644); err != nil {
		t.Fatal(err)
	}

	origShellsFile := ShellsFile
	defer func() {
		ShellsFile = origShellsFile
	}()
	ShellsFile = shellsFilePath

	d := NewDiscoverer()
	// Mock ExecLookPath to only return testCommand successfully
	d.ExecLookPath = func(file string) (string, error) {
		if file == testCommand {
			return testCommand, nil
		}
		return "", os.ErrNotExist
	}

	sessions, err := d.discoverShellSessions()
	if err != nil {
		t.Fatalf("discoverShellSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.Type != "C" {
		t.Errorf("Expected Type C, got %s", s.Type)
	}
	if s.Name != filepath.Base(testCommand) {
		t.Errorf("Expected Name %s, got %s", filepath.Base(testCommand), s.Name)
	}
	if s.Exec != testCommand {
		t.Errorf("Expected Exec %s, got %s", testCommand, s.Exec)
	}
}

func TestParseDesktopFileErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	d := NewDiscoverer()

	// Missing Exec
	path1 := filepath.Join(tmpDir, "1.desktop")
	if err := os.WriteFile(path1, missingExecContent, 0644); err != nil {
		t.Fatalf("failed writing %s: %v", path1, err)
	}
	if _, err := d.parseDesktopFile(path1, "X"); err == nil {
		t.Error("Expected error for missing Exec")
	}

	// TryExec not found
	path2 := filepath.Join(tmpDir, "2.desktop")
	if err := os.WriteFile(path2, missingTryExecContent, 0644); err != nil {
		t.Fatalf("failed writing %s: %v", path2, err)
	}
	if _, err := d.parseDesktopFile(path2, "X"); err == nil {
		t.Error("Expected error for missing TryExec binary")
	}

	// Exec binary not found (when no TryExec)
	path3 := filepath.Join(tmpDir, "3.desktop")
	if err := os.WriteFile(path3, missingExecBinContent, 0644); err != nil {
		t.Fatalf("failed writing %s: %v", path3, err)
	}
	if _, err := d.parseDesktopFile(path3, "X"); err == nil {
		t.Error("Expected error for missing Exec binary")
	}
}

func TestStripFreedesktopExecVariables(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"gnome-session %U", "gnome-session"},
		{"sway %f", "sway"},
		{"startplasma-wayland %i %c", "startplasma-wayland"},
		{"/usr/bin/some-wm --arg %F %u %k", "/usr/bin/some-wm --arg"},
		{"/bin/sh -c 'exec xterm'", "/bin/sh -c 'exec xterm'"}, // no vars
	}

	for _, c := range cases {
		got := stripFreedesktopExecVariables(c.input)
		if got != c.expected {
			t.Errorf("stripFreedesktopExecVariables(%q) = %q; expected %q", c.input, got, c.expected)
		}
	}
}
