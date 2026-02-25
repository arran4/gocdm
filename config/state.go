package config

import (
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

func SaveState(lastSession string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
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
