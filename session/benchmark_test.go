package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkDiscoverCustomSessions(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "cdm-bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a bunch of fake desktop files
	for i := 0; i < 50; i++ {
		content := fmt.Sprintf("[Desktop Entry]\nName=Session%d\nExec=/bin/true\nType=Application\n", i)
		err := os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("session%d.desktop", i)), []byte(content), 0644)
		if err != nil {
			b.Fatal(err)
		}
	}

	d := NewDiscoverer()
	// Mock ExecLookPath to avoid system calls
	d.ExecLookPath = func(s string) (string, error) {
		return s, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sessions, err := d.discoverCustomSessions(tmpDir, "X")
		if err != nil {
			b.Fatal(err)
		}
		if len(sessions) != 50 {
			b.Fatalf("expected 50 sessions, got %d", len(sessions))
		}
	}
}
