package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateLoadSave(t *testing.T) {
	// We need to mock user home dir. Set both HOME (unix) and USERPROFILE (windows)
	// so os.UserHomeDir resolves to the temp directory on all supported platforms.
	tmpHome, err := os.MkdirTemp("", "gocdm-home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	// Test load when file does not exist
	state, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if state.LastSession != "" {
		t.Errorf("Expected empty last session, got %s", state.LastSession)
	}

	// Test save
	if err := SaveState("my-session"); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Test load after save
	state, err = LoadState()
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if state.LastSession != "my-session" {
		t.Errorf("Expected last session 'my-session', got '%s'", state.LastSession)
	}
}

func TestLoadStateInvalidJSON(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "gocdm-home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	stateDir := filepath.Join(tmpHome, ".config", "gocdm")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	statePath := filepath.Join(stateDir, "state.json")
	if err := os.WriteFile(statePath, []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = LoadState()
	if err == nil {
		t.Fatal("Expected error loading invalid JSON state")
	}
}
