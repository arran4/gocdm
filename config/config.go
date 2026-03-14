package config

import (
	"bufio"
	"fmt"
	"os"
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

func DiscoverConfigPath() string {
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".cdmrc")
	if _, err := os.Stat(p); err == nil {
		return p
	}

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	p = filepath.Join(xdgConfig, "cdm", "cdmrc")
	if _, err := os.Stat(p); err == nil {
		return p
	}

	p = "/etc/cdmrc"
	if _, err := os.Stat(p); err == nil {
		return p
	}

	return ""
}

func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = DiscoverConfigPath()
	}
	if configPath == "" {
		return DefaultConfig(), nil
	}

	cfg := DefaultConfig()
	if err := parseConfigFile(configPath, cfg); err != nil {
		return nil, err
	}
	if err := validateSessionLists(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func parseConfigFile(path string, cfg *Config) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "dialogrc":
			cfg.DialogRC = trimShellQuotes(value)
		case "countfrom":
			if v, err := strconv.Atoi(trimShellQuotes(value)); err == nil {
				cfg.CountFrom = v
			}
		case "display":
			if v, err := strconv.Atoi(trimShellQuotes(value)); err == nil {
				cfg.Display = v
			}
		case "xtty":
			cfg.XTTY = trimShellQuotes(value)
		case "locktty":
			cfg.LockTTY = parseBool(trimShellQuotes(value))
		case "consolekit":
			cfg.ConsoleKit = parseBool(trimShellQuotes(value))
		case "cktimeout":
			if v, err := strconv.Atoi(trimShellQuotes(value)); err == nil {
				cfg.CKTimeout = v
			}
		case "altstartx":
			cfg.AltStartX = parseBool(trimShellQuotes(value))
		case "startxlog":
			cfg.StartXLog = trimShellQuotes(value)
		case "binlist", "namelist", "flaglist", "serverargs":
			arr, err := parseShellArray(value)
			if err != nil {
				return fmt.Errorf("config line %d (%s): %w", lineNo, key, err)
			}
			switch key {
			case "binlist":
				cfg.BinList = arr
			case "namelist":
				cfg.NameList = arr
			case "flaglist":
				cfg.FlagList = arr
			case "serverargs":
				cfg.ServerArgs = arr
			}
		}
	}
	if err := s.Err(); err != nil {
		return fmt.Errorf("scan config: %w", err)
	}
	return nil
}

func validateSessionLists(cfg *Config) error {
	if len(cfg.BinList) == 0 {
		return nil
	}
	if len(cfg.NameList) > 0 && len(cfg.NameList) != len(cfg.BinList) {
		return fmt.Errorf("namelist length %d does not match binlist length %d", len(cfg.NameList), len(cfg.BinList))
	}
	if len(cfg.FlagList) > 0 && len(cfg.FlagList) != len(cfg.BinList) {
		return fmt.Errorf("flaglist length %d does not match binlist length %d", len(cfg.FlagList), len(cfg.BinList))
	}
	return nil
}

func parseShellArray(value string) ([]string, error) {
	v := strings.TrimSpace(value)
	if !strings.HasPrefix(v, "(") || !strings.HasSuffix(v, ")") {
		return nil, fmt.Errorf("malformed array: expected parentheses")
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		return []string{}, nil
	}
	return shellSplit(inner)
}

func shellSplit(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		switch r {
		case '\\':
			escaped = true
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			} else {
				cur.WriteRune(r)
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			} else {
				cur.WriteRune(r)
			}
		case ' ', '\t':
			if inSingle || inDouble {
				cur.WriteRune(r)
			} else {
				flush()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if escaped || inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote or escape in array")
	}
	flush()
	return out, nil
}

func trimShellQuotes(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
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
