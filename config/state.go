package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var stateGeteuid = os.Geteuid

type State struct {
	LastSession string `json:"last_session"`
}

func LoadState() (*State, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &State{}, err
	}
	return LoadStateAt(home)
}

// LoadStateAt loads saved session state from home without privilege checks.
func LoadStateAt(home string) (*State, error) {
	return LoadStateForUser(home, -1)
}

// LoadStateForUser safely loads saved session state from home/.config/gocdm/state.json.
func LoadStateForUser(home string, expectedUID int) (*State, error) {
	if home == "" {
		return &State{}, fmt.Errorf("home directory not provided")
	}
	relPath := filepath.Join(".config", "gocdm", "state.json")
	data, err := SafeReadUserFile(home, relPath, expectedUID, true)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return &State{}, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return &State{}, err
	}
	return &state, nil
}

// SaveStateAt saves lastSession under home/.config/gocdm/state.json safely without symlinks.
func SaveStateAt(home, lastSession string) error {
	return SafeSaveStateAt(home, lastSession, -1)
}

// SafeSaveStateAt saves lastSession under home/.config/gocdm/state.json atomically and safely.
// When expectedUID >= 0, it refuses to write non-root user state while running as root
// and verifies directory/file ownership without mutating permissions via chown.
func SafeSaveStateAt(home, lastSession string, expectedUID int) error {
	if home == "" {
		return fmt.Errorf("home directory not provided")
	}
	if expectedUID >= 0 && stateGeteuid() == 0 && expectedUID != 0 {
		return fmt.Errorf("refusing to write user state in %q while running as root (expected uid %d)", home, expectedUID)
	}
	if err := ValidateHomeDir(home); err != nil {
		return err
	}

	state := State{
		LastSession: lastSession,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return safeSaveStatePlatform(home, data, expectedUID)
}
