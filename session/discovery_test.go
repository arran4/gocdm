package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSessions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cdm-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	x11Dir := filepath.Join(tmpDir, "X11", "Sessions")
	xsessionsDir := filepath.Join(tmpDir, "usr", "share", "xsessions")

	if err := os.MkdirAll(x11Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(xsessionsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create dummy X11 session
	if err := os.WriteFile(filepath.Join(x11Dir, "x11session"), []byte("#!/bin/sh\necho x11"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create dummy XSession .desktop file
	// We use /bin/sh which should exist
	desktopContent := `[Desktop Entry]
Name=Test Session
Exec=/bin/sh -c "echo test"
`
	if err := os.WriteFile(filepath.Join(xsessionsDir, "test.desktop"), []byte(desktopContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Save original vars
	origX11 := X11SessionsDir
	origXSessions := XSessionsDir
	defer func() {
		X11SessionsDir = origX11
		XSessionsDir = origXSessions
	}()

	X11SessionsDir = x11Dir
	XSessionsDir = xsessionsDir

	// Test X11 Sessions (priority)
	sessions, err := DiscoverSessions()
	if err != nil {
		t.Fatalf("DiscoverSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}
	if len(sessions) > 0 && sessions[0].Name != "x11session" {
		t.Errorf("Expected session name 'x11session', got '%s'", sessions[0].Name)
	}

	// Remove X11 session to test XSessions fallback
	os.Remove(filepath.Join(x11Dir, "x11session"))

	sessions, err = DiscoverSessions()
	if err != nil {
		t.Fatalf("DiscoverSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}
	if len(sessions) > 0 && sessions[0].Name != "Test Session" {
		t.Errorf("Expected session name 'Test Session', got '%s'", sessions[0].Name)
	}
}
