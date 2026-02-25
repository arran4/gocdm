package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	DialogRC   string
	CountFrom  int
	Display    int
	XTTY       string // "keep" or number
	LockTTY    bool
	ConsoleKit bool
	CKTimeout  int
	AltStartX  bool
	StartXLog  string
	BinList    []string
	NameList   []string
	FlagList   []string
	ServerArgs []string
}

func LoadConfig(configPath string) (*Config, error) {
	// If configPath is empty, try default locations
	if configPath == "" {
		home, _ := os.UserHomeDir()
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(home, ".config")
		}

		paths := []string{
			filepath.Join(home, ".cdmrc"),
			filepath.Join(xdgConfig, "cdm", "cdmrc"),
			"/etc/cdmrc",
		}

		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				configPath = p
				break
			}
		}
	}

	if configPath == "" {
		// No config found, return defaults
		return DefaultConfig(), nil
	}

	cmdStr := `
source "$1"
echo "dialogrc=$dialogrc"
echo "countfrom=$countfrom"
echo "display=$display"
echo "xtty=$xtty"
echo "locktty=$locktty"
echo "consolekit=$consolekit"
echo "cktimeout=$cktimeout"
echo "altstartx=$altstartx"
echo "startxlog=$startxlog"

for i in "${!binlist[@]}"; do echo "binlist_$i=${binlist[$i]}"; done
for i in "${!namelist[@]}"; do echo "namelist_$i=${namelist[$i]}"; done
for i in "${!flaglist[@]}"; do echo "flaglist_$i=${flaglist[$i]}"; done
for i in "${!serverargs[@]}"; do echo "serverargs_$i=${serverargs[$i]}"; done
`

	cmd := exec.Command("bash", "-c", cmdStr, "--", configPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to source config: %w", err)
	}

	cfg := DefaultConfig()
	scanner := bufio.NewScanner(bytes.NewReader(output))

	// Temporary maps for arrays
	binListMap := make(map[int]string)
	nameListMap := make(map[int]string)
	flagListMap := make(map[int]string)
	serverArgsMap := make(map[int]string)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := parts[1]

		switch key {
		case "dialogrc":
			cfg.DialogRC = value
		case "countfrom":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.CountFrom = v
			}
		case "display":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.Display = v
			}
		case "xtty":
			cfg.XTTY = value
		case "locktty":
			cfg.LockTTY = parseBool(value)
		case "consolekit":
			cfg.ConsoleKit = parseBool(value)
		case "cktimeout":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.CKTimeout = v
			}
		case "altstartx":
			cfg.AltStartX = parseBool(value)
		case "startxlog":
			cfg.StartXLog = value
		default:
			if strings.HasPrefix(key, "binlist_") {
				idx, _ := strconv.Atoi(strings.TrimPrefix(key, "binlist_"))
				binListMap[idx] = value
			} else if strings.HasPrefix(key, "namelist_") {
				idx, _ := strconv.Atoi(strings.TrimPrefix(key, "namelist_"))
				nameListMap[idx] = value
			} else if strings.HasPrefix(key, "flaglist_") {
				idx, _ := strconv.Atoi(strings.TrimPrefix(key, "flaglist_"))
				flagListMap[idx] = value
			} else if strings.HasPrefix(key, "serverargs_") {
				idx, _ := strconv.Atoi(strings.TrimPrefix(key, "serverargs_"))
				serverArgsMap[idx] = value
			}
		}
	}

	if len(binListMap) > 0 {
		cfg.BinList = mapToSlice(binListMap)
	}
	if len(nameListMap) > 0 {
		cfg.NameList = mapToSlice(nameListMap)
	}
	if len(flagListMap) > 0 {
		cfg.FlagList = mapToSlice(flagListMap)
	}
	if len(serverArgsMap) > 0 {
		cfg.ServerArgs = mapToSlice(serverArgsMap)
	}

	return cfg, nil
}

func DefaultConfig() *Config {
	return &Config{
		CountFrom:  0,
		Display:    0,
		XTTY:       "7",
		LockTTY:    false,
		ConsoleKit: true,
		CKTimeout:  30,
		AltStartX:  false,
		StartXLog:  "/dev/null",
		ServerArgs: []string{"-nolisten", "tcp"},
	}
}

func parseBool(v string) bool {
	v = strings.ToLower(v)
	return v == "yes" || v == "true" || v == "on" || v == "1"
}

func mapToSlice(m map[int]string) []string {
	if len(m) == 0 {
		return nil
	}
	maxIdx := -1
	for k := range m {
		if k > maxIdx {
			maxIdx = k
		}
	}
	s := make([]string, maxIdx+1)
	for k, v := range m {
		s[k] = v
	}
	return s
}
