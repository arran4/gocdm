package config

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var envVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var localeEnvAllowlist = []string{
	"LANG",
	"LANGUAGE",
	"LC_ALL",
	"LC_CTYPE",
	"LC_NUMERIC",
	"LC_TIME",
	"LC_COLLATE",
	"LC_MONETARY",
	"LC_MESSAGES",
	"LC_PAPER",
	"LC_NAME",
	"LC_ADDRESS",
	"LC_TELEPHONE",
	"LC_MEASUREMENT",
	"LC_IDENTIFICATION",
	"TZ",
}

// GettyEnvOptions configures deterministic session environment construction in getty login mode.
type GettyEnvOptions struct {
	LauncherEnv    []string
	Account        *AccountInfo
	PamEnvFile     string
	PamSessionEnv  []string
	SessionType    string // "W", "X", or "C"
	SessionName    string
	DesktopSession string
	CurrentDesktop string
	WorkingDir     string
	SpawnPID       int
}

// UserEnvOptions configures environment updates in already-authenticated user session mode.
type UserEnvOptions struct {
	BaseEnv        []string
	SessionType    string // "W", "X", or "C"
	SessionName    string
	DesktopSession string
	CurrentDesktop string
	SpawnPID       int
}

// BuildGettySessionEnv builds an isolated session environment for an authenticated user in direct getty mode.
// Only allowlisted terminal/locale variables are inherited from LauncherEnv; canonical identity, safe PATH,
// pam_env.conf, authoritative PAM session environment, and session metadata are applied deterministically.
func BuildGettySessionEnv(opts GettyEnvOptions) ([]string, error) {
	if opts.Account == nil || opts.Account.Username == "" {
		return nil, fmt.Errorf("authenticated account info is required")
	}

	launcherMap := EnvSliceToMap(opts.LauncherEnv)
	envMap := make(map[string]string)

	if termVal := launcherMap["TERM"]; termVal != "" {
		envMap["TERM"] = termVal
	} else {
		envMap["TERM"] = "linux"
	}

	for _, key := range localeEnvAllowlist {
		if val := launcherMap[key]; val != "" {
			envMap[key] = val
		}
	}

	if opts.Account.UID == 0 {
		envMap["PATH"] = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	} else {
		envMap["PATH"] = "/usr/local/bin:/usr/bin:/bin"
	}

	shell := shellOrDefault(opts.Account.Shell)
	envMap["USER"] = opts.Account.Username
	envMap["LOGNAME"] = opts.Account.Username
	envMap["HOME"] = opts.Account.HomeDir
	envMap["SHELL"] = shell

	if opts.PamEnvFile != "" {
		if err := applyPamEnvFile(envMap, opts.PamEnvFile); err != nil {
			return nil, err
		}
	}

	for k, v := range EnvSliceToMap(opts.PamSessionEnv) {
		envMap[k] = v
	}

	// Ensure canonical identity remains populated if PAM left any entry empty.
	if envMap["USER"] == "" {
		envMap["USER"] = opts.Account.Username
	}
	if envMap["LOGNAME"] == "" {
		envMap["LOGNAME"] = opts.Account.Username
	}
	if envMap["HOME"] == "" {
		envMap["HOME"] = opts.Account.HomeDir
	}
	if envMap["SHELL"] == "" {
		envMap["SHELL"] = shell
	}

	applySessionTypeEnv(envMap, opts.SessionType, opts.SessionName, opts.DesktopSession, opts.CurrentDesktop)

	pwd := opts.WorkingDir
	if pwd == "" {
		pwd = opts.Account.HomeDir
	}
	if pwd != "" {
		envMap["PWD"] = pwd
	}

	if opts.SpawnPID > 0 {
		envMap["GOCDM_SPAWN"] = strconv.Itoa(opts.SpawnPID)
	}

	return EnvMapToSlice(envMap), nil
}

// BuildUserSessionEnv enriches an already-authenticated user environment with session metadata.
func BuildUserSessionEnv(opts UserEnvOptions) []string {
	envMap := EnvSliceToMap(opts.BaseEnv)
	applySessionTypeEnv(envMap, opts.SessionType, opts.SessionName, opts.DesktopSession, opts.CurrentDesktop)
	if opts.SpawnPID > 0 {
		envMap["GOCDM_SPAWN"] = strconv.Itoa(opts.SpawnPID)
	}
	return EnvMapToSlice(envMap)
}

// SetEnvValue returns a copy of env with key set to value.
func SetEnvValue(env []string, key, value string) []string {
	envMap := EnvSliceToMap(env)
	envMap[key] = value
	return EnvMapToSlice(envMap)
}

func applySessionTypeEnv(envMap map[string]string, sessionType, sessionName, desktopSession, currentDesktop string) {
	stype := strings.ToUpper(strings.TrimSpace(sessionType))
	switch {
	case strings.HasPrefix(stype, "W"):
		envMap["XDG_SESSION_TYPE"] = "wayland"
		envMap["XDG_SESSION_CLASS"] = "user"
		setDesktopEnvVars(envMap, sessionName, desktopSession, currentDesktop)
	case strings.HasPrefix(stype, "X"):
		envMap["XDG_SESSION_TYPE"] = "x11"
		envMap["XDG_SESSION_CLASS"] = "user"
		setDesktopEnvVars(envMap, sessionName, desktopSession, currentDesktop)
	case strings.HasPrefix(stype, "C"):
		envMap["XDG_SESSION_TYPE"] = "tty"
		envMap["XDG_SESSION_CLASS"] = "user"
	}
}

func setDesktopEnvVars(envMap map[string]string, sessionName, desktopSession, currentDesktop string) {
	sessDesktop := desktopSession
	if sessDesktop == "" {
		sessDesktop = sessionName
	}
	curDesktop := currentDesktop
	if curDesktop == "" {
		curDesktop = sessDesktop
	}
	if sessDesktop != "" {
		envMap["XDG_SESSION_DESKTOP"] = sessDesktop
		envMap["DESKTOP_SESSION"] = sessDesktop
	}
	if curDesktop != "" {
		envMap["XDG_CURRENT_DESKTOP"] = curDesktop
	}
}

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
	UID      int
	GID      int
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
	uid, err := strconv.Atoi(parts[2])
	if err != nil || uid < 0 {
		return nil
	}
	gid, err := strconv.Atoi(parts[3])
	if err != nil || gid < 0 {
		return nil
	}
	return &PasswdEntry{
		Username: parts[0],
		UID:      uid,
		GID:      gid,
		HomeDir:  parts[5],
		Shell:    parts[6],
	}
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
	keys := make([]string, 0, len(envMap))
	for key := range envMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+envMap[key])
	}
	return out
}
