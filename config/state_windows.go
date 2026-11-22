//go:build windows

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func safeSaveStatePlatform(home string, data []byte, expectedUID int) error {
	info, err := os.Lstat(home)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("home path %q is not a directory", home)
	}
	configDir := filepath.Join(home, ".config", "gocdm")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	statePath := filepath.Join(configDir, "state.json")
	if st, err := os.Lstat(statePath); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlink state file %q", statePath)
	}
	return os.WriteFile(statePath, data, 0o600)
}
