package session

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var (
	X11SessionsDir     = "/etc/X11/Sessions"
	XSessionsDir       = "/usr/share/xsessions"
	WaylandSessionsDir = "/usr/share/wayland-sessions"
)

type Session struct {
	Name string
	Exec string
	Type string // "X", "C", or "W" (Wayland)
}

func DiscoverSessions(userHome string) ([]Session, error) {
	var sessions []Session
	seen := make(map[string]bool)

	addSession := func(s Session) {
		key := s.Name + "|" + s.Exec
		if !seen[key] {
			sessions = append(sessions, s)
			seen[key] = true
		}
	}

	// 1. User specific sessions (Highest priority)
	// ~/.xsession, ~/.xinitrc
	userSessions, err := discoverUserSessions(userHome)
	if err == nil {
		for _, s := range userSessions {
			addSession(s)
		}
	}

	// ~/.local/share/xsessions
	localXSessions, err := discoverCustomSessions(filepath.Join(userHome, ".local", "share", "xsessions"), "X")
	if err == nil {
		for _, s := range localXSessions {
			addSession(s)
		}
	}

	// ~/.config/wayland-sessions
	localWaylandSessions, err := discoverCustomSessions(filepath.Join(userHome, ".config", "wayland-sessions"), "W")
	if err == nil {
		for _, s := range localWaylandSessions {
			addSession(s)
		}
	}

	// 2. System sessions
	// Try /etc/X11/Sessions (Legacy)
	legacySessions, err := discoverX11Sessions()
	if err == nil {
		for _, s := range legacySessions {
			addSession(s)
		}
	}

	// Try /usr/share/xsessions
	xSessions, err := discoverXSessions()
	if err == nil {
		for _, s := range xSessions {
			addSession(s)
		}
	}

	// Try /usr/share/wayland-sessions
	wSessions, err := discoverWaylandSessions()
	if err == nil {
		for _, s := range wSessions {
			addSession(s)
		}
	}

	// Sort sessions by name
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].Name < sessions[j].Name
	})

	return sessions, nil
}

func discoverUserSessions(home string) ([]Session, error) {
	var sessions []Session

	checkFile := func(filename, name string) {
		path := filepath.Join(home, filename)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			// Check if executable? Usually shells source them, but let's assume if it exists it's runnable via sh
			// Or check execute bit?
			// Standard says .xinitrc is a shell script.
			// Let's assume it's a valid session if it exists.
			// Type X for .xinitrc/.xsession
			sessions = append(sessions, Session{
				Name: name,
				Exec: path,
				Type: "X",
			})
		}
	}

	checkFile(".xinitrc", "Custom X Session (.xinitrc)")
	checkFile(".xsession", "Custom X Session (.xsession)")

	return sessions, nil
}

func discoverCustomSessions(dir string, sessionType string) ([]Session, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []Session
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".desktop") {
			session, err := parseDesktopFile(filepath.Join(dir, entry.Name()), sessionType)
			if err == nil {
				sessions = append(sessions, session)
			}
		}
	}
	return sessions, nil
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
	return discoverCustomSessions(XSessionsDir, "X")
}

func discoverWaylandSessions() ([]Session, error) {
	return discoverCustomSessions(WaylandSessionsDir, "W")
}

func parseDesktopFile(path string, defaultType string) (Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return Session{}, err
	}
	defer file.Close()

	var execCmd, name, tryExec string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Exec=") {
			execCmd = strings.TrimPrefix(line, "Exec=")
		} else if strings.HasPrefix(line, "Name=") {
			name = strings.TrimPrefix(line, "Name=")
		} else if strings.HasPrefix(line, "TryExec=") {
			tryExec = strings.TrimPrefix(line, "TryExec=")
		}
	}

	if execCmd == "" || name == "" {
		return Session{}, fmt.Errorf("missing Exec or Name")
	}

	// Check TryExec if present
	if tryExec != "" {
		if _, err := exec.LookPath(tryExec); err != nil {
			return Session{}, fmt.Errorf("TryExec binary not found: %s", tryExec)
		}
	}

	// Check if executable in Exec exists (if not checked by TryExec)
	// Some Exec lines are complex, e.g. "gnome-session --session=gnome"
	// We only check the first token.
	if tryExec == "" {
		cmdParts := strings.Fields(execCmd)
		if len(cmdParts) > 0 {
			bin := cmdParts[0]
			// Only check if it's an absolute path or in PATH
			// Some desktop files use full paths, some use binaries in PATH.
			if _, err := exec.LookPath(bin); err != nil {
				return Session{}, fmt.Errorf("executable not found: %s", bin)
			}
		}
	}

	return Session{
		Name: name,
		Exec: execCmd,
		Type: defaultType,
	}, nil
}
