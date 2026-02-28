package session

import (
	"os"
	"path/filepath"
	"testing"
	_ "embed"
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
	desktopContent := `[Desktop Entry]
Name=Standard X Session
Exec=/bin/sh -c "echo standard"
`
	if err := os.WriteFile(filepath.Join(xsessionsDir, "standard.desktop"), []byte(desktopContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Wayland Session .desktop
	waylandContent := `[Desktop Entry]
Name=Wayland Session
Exec=/bin/sh -c "echo wayland"
`
	if err := os.WriteFile(filepath.Join(waylandSessionsDir, "wayland.desktop"), []byte(waylandContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. User .xinitrc
	if err := os.WriteFile(filepath.Join(userHome, ".xinitrc"), []byte("#!/bin/sh\necho user"), 0755); err != nil {
		t.Fatal(err)
	}

	// 5. User Custom Wayland Session
	userWaylandContent := `[Desktop Entry]
Name=User Wayland
Exec=/bin/sh -c "echo userwayland"
`
	if err := os.WriteFile(filepath.Join(userConfigWayland, "user-wayland.desktop"), []byte(userWaylandContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Save original vars
	origX11 := X11SessionsDir
	origXSessions := XSessionsDir
	origWaylandSessions := WaylandSessionsDir
	defer func() {
		X11SessionsDir = origX11
		XSessionsDir = origXSessions
		WaylandSessionsDir = origWaylandSessions
	}()

	X11SessionsDir = x11Dir
	XSessionsDir = xsessionsDir
	WaylandSessionsDir = waylandSessionsDir

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

	expectedCount := 5
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
	}

	for name, typ := range expectations {
		if gotType, ok := found[name]; !ok {
			t.Errorf("Expected session '%s' not found", name)
		} else if gotType != typ {
			t.Errorf("Expected session '%s' to have type '%s', got '%s'", name, typ, gotType)
		}
	}
}

func TestParseDesktopFileErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Missing Exec
	path1 := filepath.Join(tmpDir, "1.desktop")
	os.WriteFile(path1, missingExecContent, 0644)
	if _, err := parseDesktopFile(path1, "X"); err == nil {
		t.Error("Expected error for missing Exec")
	}

	// TryExec not found
	path2 := filepath.Join(tmpDir, "2.desktop")
	os.WriteFile(path2, missingTryExecContent, 0644)
	if _, err := parseDesktopFile(path2, "X"); err == nil {
		t.Error("Expected error for missing TryExec binary")
	}

	// Exec binary not found (when no TryExec)
	path3 := filepath.Join(tmpDir, "3.desktop")
	os.WriteFile(path3, missingExecBinContent, 0644)
	if _, err := parseDesktopFile(path3, "X"); err == nil {
		t.Error("Expected error for missing Exec binary")
	}
}
