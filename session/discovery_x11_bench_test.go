package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkDiscoverX11Sessions(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "cdm-bench-x11")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		b.Fatal(err)
	}

	// Create 100 dummy X11 session files
	for i := 0; i < 100; i++ {
		_ = os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("bench%d", i)), []byte("test"), 0755)
	}

	origX11 := X11SessionsDir
	defer func() {
		X11SessionsDir = origX11
	}()
	X11SessionsDir = tmpDir

	d := NewDiscoverer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.discoverX11Sessions()
	}
}
