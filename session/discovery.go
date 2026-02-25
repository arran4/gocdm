package session

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	X11SessionsDir = "/etc/X11/Sessions"
	XSessionsDir   = "/usr/share/xsessions"
)

type Session struct {
	Name string
	Exec string
	Type string // "X" or "C"
}

func DiscoverSessions() ([]Session, error) {
	// Try /etc/X11/Sessions
	sessions, err := discoverX11Sessions()
	if err == nil && len(sessions) > 0 {
		return sessions, nil
	}

	// Try /usr/share/xsessions
	return discoverXSessions()
}

func discoverX11Sessions() ([]Session, error) {
	dir := X11SessionsDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []Session
	for _, entry := range entries {
		if !entry.IsDir() {
			sessions = append(sessions, Session{
				Name: entry.Name(),
				Exec: filepath.Join(dir, entry.Name()),
				Type: "X",
			})
		}
	}
	return sessions, nil
}

func discoverXSessions() ([]Session, error) {
	dir := XSessionsDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []Session
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".desktop") {
			session, err := parseDesktopFile(filepath.Join(dir, entry.Name()))
			if err == nil {
				sessions = append(sessions, session)
			}
		}
	}
	return sessions, nil
}

func parseDesktopFile(path string) (Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return Session{}, err
	}
	defer file.Close()

	var execCmd, name string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Exec=") {
			execCmd = strings.TrimPrefix(line, "Exec=")
		} else if strings.HasPrefix(line, "Name=") {
			name = strings.TrimPrefix(line, "Name=")
		}
	}

	if execCmd == "" || name == "" {
		return Session{}, fmt.Errorf("missing Exec or Name")
	}

	// Check if executable exists
	cmdParts := strings.Fields(execCmd)
	if len(cmdParts) == 0 {
		return Session{}, fmt.Errorf("empty Exec")
	}
	bin := cmdParts[0]
	if _, err := exec.LookPath(bin); err != nil {
		return Session{}, fmt.Errorf("executable not found: %s", bin)
	}

	return Session{
		Name: name,
		Exec: execCmd,
		Type: "X",
	}, nil
}
