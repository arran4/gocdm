package bindings

import (
	"encoding/json"

	"github.com/arran4/gocdm/config"
	"github.com/arran4/gocdm/session"
)

type response struct {
	OK    bool            `json:"ok"`
	Error string          `json:"error,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// LoadConfigJSON loads a GoCDM config file and returns a JSON-encoded response.
func LoadConfigJSON(configPath string) string {
	cfg, err := config.LoadConfig(configPath)
	return marshal(cfg, err)
}

// DefaultConfigJSON returns the default GoCDM config as a JSON-encoded response.
func DefaultConfigJSON() string {
	return marshal(config.DefaultConfig(), nil)
}

// DiscoverSessionsJSON discovers sessions for a user home and returns a JSON-encoded response.
func DiscoverSessionsJSON(userHome string) string {
	sessions, err := session.DiscoverSessions(userHome)
	return marshal(sessions, err)
}

func marshal(data any, err error) string {
	res := response{OK: err == nil}
	if err != nil {
		res.Error = err.Error()
	} else {
		dataBytes, marshalErr := json.Marshal(data)
		if marshalErr != nil {
			res.OK = false
			res.Error = marshalErr.Error()
		} else {
			res.Data = dataBytes
		}
	}

	encoded, encodeErr := json.Marshal(res)
	if encodeErr != nil {
		return `{"ok":false,"error":"failed to marshal response"}`
	}
	return string(encoded)
}
