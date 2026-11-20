//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSafeReadUserFileRejectsFIFO(t *testing.T) {
	home := t.TempDir()
	fifoPath := filepath.Join(home, ".cdmrc")
	if err := unix.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatalf("Mkfifo failed: %v", err)
	}

	_, err := SafeReadUserFile(home, ".cdmrc", os.Geteuid(), false)
	if err == nil || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("expected non-regular file refusal for FIFO, got %v", err)
	}
}
