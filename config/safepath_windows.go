//go:build windows

package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxSafeUserFileSize = 1 << 20 // 1 MiB

var safePathGeteuid = os.Geteuid

func validateRelUserPath(relPath string) ([]string, error) {
	if relPath == "" {
		return nil, fmt.Errorf("relative path cannot be empty")
	}
	if strings.ContainsRune(relPath, '\x00') {
		return nil, fmt.Errorf("relative path contains NUL byte")
	}
	if filepath.IsAbs(relPath) || strings.HasPrefix(relPath, "/") || strings.HasPrefix(relPath, "\\") {
		return nil, fmt.Errorf("path %q must be relative", relPath)
	}
	segs := strings.Split(filepath.ToSlash(relPath), "/")
	for _, seg := range segs {
		if seg == "" || seg == "." || seg == ".." {
			return nil, fmt.Errorf("invalid path segment %q in %q", seg, relPath)
		}
	}
	return segs, nil
}

// SafeReadUserFile validates paths and reads relPath beneath homeDir on Windows.
func SafeReadUserFile(homeDir, relPath string, expectedUID int, allowRootOwned bool) ([]byte, error) {
	if err := ValidateHomeDir(homeDir); err != nil {
		return nil, err
	}
	if _, err := validateRelUserPath(relPath); err != nil {
		return nil, err
	}
	fullPath := filepath.Join(homeDir, relPath)
	info, err := os.Lstat(fullPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to follow symlink %q", fullPath)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular file %q", fullPath)
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxSafeUserFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSafeUserFileSize {
		return nil, fmt.Errorf("user file %q exceeds %d bytes", fullPath, maxSafeUserFileSize)
	}
	return data, nil
}
