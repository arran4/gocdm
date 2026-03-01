package session

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func setupPathExecutable(t *testing.T, root string) string {
	t.Helper()

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	name := "test-session-cmd"
	contents := "#!/bin/sh\nexit 0\n"
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		name += ".cmd"
		contents = "@echo off\r\nexit /b 0\r\n"
		mode = 0o644
	}

	cmdPath := filepath.Join(binDir, name)
	if err := os.WriteFile(cmdPath, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}

	pathSep := string(os.PathListSeparator)
	t.Setenv("PATH", binDir+pathSep+os.Getenv("PATH"))
	if runtime.GOOS == "windows" {
		return "test-session-cmd"
	}
	return name
}
