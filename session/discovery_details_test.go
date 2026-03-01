package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoveryDetails(t *testing.T) {
	// Setup test environment (same as TestDiscoverSessions)
	tmpDir, err := os.MkdirTemp("", "cdm-test-details")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	testCommand := setupPathExecutable(t, tmpDir)

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
	legacyPath := filepath.Join(x11Dir, "legacy_x11")
	if err := os.WriteFile(legacyPath, []byte("#!/bin/sh\necho legacy"), 0755); err != nil {
		t.Fatal(err)
	}

	// 2. Standard XSession .desktop
	standardPath := filepath.Join(xsessionsDir, "standard.desktop")
	desktopContent := "[Desktop Entry]\n" +
		"Name=Standard X Session\n" +
		"Exec=" + testCommand + "\n"
	if err := os.WriteFile(standardPath, []byte(desktopContent), 0644); err != nil {
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

	// Verify Path field
	foundLegacy := false
	foundStandard := false

	for _, s := range sessions {
		if s.Name == "legacy_x11" {
			foundLegacy = true
			if s.Path != legacyPath {
				t.Errorf("legacy_x11 path mismatch: expected %s, got %s", legacyPath, s.Path)
			}
		}
		if s.Name == "Standard X Session" {
			foundStandard = true
			if s.Path != standardPath {
				t.Errorf("Standard X Session path mismatch: expected %s, got %s", standardPath, s.Path)
			}
		}
	}

	if !foundLegacy {
		t.Error("legacy_x11 session not found")
	}
	if !foundStandard {
		t.Error("Standard X Session session not found")
	}
}
