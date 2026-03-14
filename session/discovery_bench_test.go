package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkDiscoverSessions(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "cdm-bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	xsessionsDir := filepath.Join(tmpDir, "usr", "share", "xsessions")
	if err := os.MkdirAll(xsessionsDir, 0755); err != nil {
		b.Fatal(err)
	}

	// Create 100 dummy desktop files
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("[Desktop Entry]\nName=BenchSession%d\nExec=ls\n", i)
		os.WriteFile(filepath.Join(xsessionsDir, fmt.Sprintf("bench%d.desktop", i)), []byte(content), 0644)
	}

	origXSessions := XSessionsDir
	origX11 := X11SessionsDir
	origWayland := WaylandSessionsDir
	defer func() {
		XSessionsDir = origXSessions
		X11SessionsDir = origX11
		WaylandSessionsDir = origWayland
	}()
	XSessionsDir = xsessionsDir
	X11SessionsDir = "/tmp/does-not-exist-x11"
	WaylandSessionsDir = "/tmp/does-not-exist-wayland"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DiscoverSessions("/tmp/does-not-exist-home")
	}
}
