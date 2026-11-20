package config

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// AccountInfo holds the canonical OS account identity for an authenticated user.
type AccountInfo struct {
	Username string
	UID      int
	GID      int
	Groups   []int
	HomeDir  string
	Shell    string
}

func shellOrDefault(shell string) string {
	if strings.TrimSpace(shell) == "" {
		return "/bin/sh"
	}
	return shell
}

// ValidateHomeDir verifies that homeDir is a clean absolute path with no NUL bytes or parent traversal.
func ValidateHomeDir(homeDir string) error {
	if homeDir == "" {
		return fmt.Errorf("home directory is empty")
	}
	if strings.ContainsRune(homeDir, '\x00') {
		return fmt.Errorf("home directory contains NUL byte")
	}
	if !filepath.IsAbs(homeDir) {
		return fmt.Errorf("home directory %q must be an absolute path", homeDir)
	}
	for _, seg := range strings.Split(filepath.ToSlash(homeDir), "/") {
		if seg == ".." {
			return fmt.Errorf("home directory %q cannot contain '..' segments", homeDir)
		}
	}
	return nil
}

// LookupAccount resolves the canonical OS account metadata for username.
// When passwdFile is explicitly set to a custom file (not empty and not /etc/passwd),
// that file is used directly (for deterministic testing). Otherwise OS NSS lookup
// via os/user.Lookup is used with /etc/passwd queried for the login shell.
func LookupAccount(username, passwdFile string) (*AccountInfo, error) {
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if strings.ContainsAny(username, ":/\x00 \t\r\n") {
		return nil, fmt.Errorf("invalid username %q", username)
	}

	if passwdFile != "" && passwdFile != "/etc/passwd" {
		entry, err := LoadPasswdEntry(passwdFile, username)
		if err != nil {
			return nil, err
		}
		return &AccountInfo{
			Username: entry.Username,
			UID:      entry.UID,
			GID:      entry.GID,
			Groups:   []int{entry.GID},
			HomeDir:  entry.HomeDir,
			Shell:    shellOrDefault(entry.Shell),
		}, nil
	}

	u, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("lookup user %q: %w", username, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil || uid < 0 {
		return nil, fmt.Errorf("invalid uid %q for user %q", u.Uid, username)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil || gid < 0 {
		return nil, fmt.Errorf("invalid gid %q for user %q", u.Gid, username)
	}

	gidsStr, err := u.GroupIds()
	if err != nil {
		return nil, fmt.Errorf("get group ids for %q: %w", username, err)
	}
	groups := make([]int, 0, len(gidsStr)+1)
	for _, gs := range gidsStr {
		ig, err := strconv.Atoi(gs)
		if err != nil || ig < 0 {
			return nil, fmt.Errorf("invalid supplementary gid %q for user %q", gs, username)
		}
		groups = append(groups, ig)
	}
	if len(groups) == 0 {
		groups = append(groups, gid)
	}

	passwdPath := "/etc/passwd"
	if passwdFile != "" {
		passwdPath = passwdFile
	}
	shell := ""
	if entry, err := LoadPasswdEntry(passwdPath, u.Username); err == nil {
		shell = entry.Shell
	}

	return &AccountInfo{
		Username: u.Username,
		UID:      uid,
		GID:      gid,
		Groups:   groups,
		HomeDir:  u.HomeDir,
		Shell:    shellOrDefault(shell),
	}, nil
}
