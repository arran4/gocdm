package config

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var envVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// BuildSessionEnv enriches baseEnv with user identity from passwdFile and optional
// overrides from pamEnvFile.
func BuildSessionEnv(baseEnv []string, username, passwdFile, pamEnvFile string) ([]string, error) {
	envMap := EnvSliceToMap(baseEnv)

	if username != "" {
		entry, err := LoadPasswdEntry(passwdFile, username)
		if err != nil {
			return nil, err
		}
		envMap["USER"] = entry.Username
		envMap["LOGNAME"] = entry.Username
		envMap["HOME"] = entry.HomeDir
		envMap["SHELL"] = entry.Shell
	}

	if pamEnvFile != "" {
		if err := applyPamEnvFile(envMap, pamEnvFile); err != nil {
			return nil, err
		}
	}

	return EnvMapToSlice(envMap), nil
}

type PasswdEntry struct {
	Username string
	HomeDir  string
	Shell    string
}

func LoadPasswdEntry(path, username string) (*PasswdEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open passwd file: %w", err)
	}
	defer f.Close()

	prefix := username + ":"
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		if entry := parsePasswdLine(line); entry != nil {
			return entry, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan passwd file: %w", err)
	}
	return nil, fmt.Errorf("user %q not found in passwd file", username)
}

func parsePasswdLine(line string) *PasswdEntry {
	parts := strings.Split(line, ":")
	if len(parts) < 7 {
		return nil
	}
	return &PasswdEntry{Username: parts[0], HomeDir: parts[5], Shell: parts[6]}
}

func applyPamEnvFile(envMap map[string]string, path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open pam env file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		if !envVarNamePattern.MatchString(name) {
			continue
		}

		var defaultVal string
		var overrideVal string
		for _, field := range fields[1:] {
			if strings.HasPrefix(field, "DEFAULT=") {
				defaultVal = strings.TrimPrefix(field, "DEFAULT=")
			}
			if strings.HasPrefix(field, "OVERRIDE=") {
				overrideVal = strings.TrimPrefix(field, "OVERRIDE=")
			}
		}

		if overrideVal != "" {
			envMap[name] = expandEnvValue(unquote(overrideVal), envMap)
			continue
		}
		if _, exists := envMap[name]; !exists && defaultVal != "" {
			envMap[name] = expandEnvValue(unquote(defaultVal), envMap)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan pam env file: %w", err)
	}
	return nil
}

func expandEnvValue(v string, envMap map[string]string) string {
	return os.Expand(v, func(name string) string {
		if val, ok := envMap[name]; ok {
			return val
		}
		return ""
	})
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
		return v[1 : len(v)-1]
	}
	return v
}

func EnvSliceToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		m[parts[0]] = parts[1]
	}
	return m
}

func EnvMapToSlice(envMap map[string]string) []string {
	out := make([]string, 0, len(envMap))
	for key, val := range envMap {
		out = append(out, key+"="+val)
	}
	return out
}
