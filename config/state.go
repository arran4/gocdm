package config

import (
	"fmt"
	"encoding/json"
	"os"
	"path/filepath"
)

type State struct {
	LastSession string `json:"last_session"`
}

func LoadState() (*State, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &State{}, err
	}

	statePath := filepath.Join(home, ".config", "gocdm", "state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return &State{}, nil
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		return &State{}, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return &State{}, err
	}

	return &state, nil
}

func SaveStateAt(home, lastSession string) error {
	if home == "" {
		return fmt.Errorf("home directory not provided")
	}

	configDir := filepath.Join(home, ".config", "gocdm")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	state := State{
		LastSession: lastSession,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	statePath := filepath.Join(configDir, "state.json")
	return os.WriteFile(statePath, data, 0644)
}
