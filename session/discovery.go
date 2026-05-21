package session

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var (
	X11SessionsDir           = "/etc/X11/Sessions"
	XSessionsDir             = "/usr/share/xsessions"
	WaylandSessionsDir       = "/usr/share/wayland-sessions"
	PlasmaWaylandWrapperPath = "/usr/libexec/plasma-dbus-run-session-if-needed"
)

type Session struct {
	Name string
	Exec string
	Type string // "X", "C", or "W" (Wayland)
	Path string
}

type Discoverer struct {
	ExecLookPath func(string) (string, error)
}

func NewDiscoverer() *Discoverer {
	var mu sync.Mutex
	type result struct {
		path string
		err  error
	}
	cache := make(map[string]result)

	return &Discoverer{
		ExecLookPath: func(file string) (string, error) {
			mu.Lock()
			res, ok := cache[file]
			mu.Unlock()

			if ok {
				return res.path, res.err
			}

			path, err := exec.LookPath(file)
			mu.Lock()
			cache[file] = result{path, err}
			mu.Unlock()
			return path, err
		},
	}
}

func DiscoverSessions(userHome string) ([]Session, error) {
	return NewDiscoverer().Discover(userHome)
}

func (d *Discoverer) Discover(userHome string) ([]Session, error) {
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
	userSessions, err := d.discoverUserSessions(userHome)
	if err == nil {
		for _, s := range userSessions {
			addSession(s)
		}
	}

	// ~/.local/share/xsessions
	localXSessions, err := d.discoverCustomSessions(filepath.Join(userHome, ".local", "share", "xsessions"), "X")
	if err == nil {
		for _, s := range localXSessions {
			addSession(s)
		}
	}

	// ~/.config/wayland-sessions
	localWaylandSessions, err := d.discoverCustomSessions(filepath.Join(userHome, ".config", "wayland-sessions"), "W")
	if err == nil {
		for _, s := range localWaylandSessions {
			addSession(s)
		}
	}

	// 2. System sessions
	// Try /etc/X11/Sessions (Legacy)
	legacySessions, err := d.discoverX11Sessions()
	if err == nil {
		for _, s := range legacySessions {
			addSession(s)
		}
	}

	// Try /usr/share/xsessions
	xSessions, err := d.discoverXSessions()
	if err == nil {
		for _, s := range xSessions {
			addSession(s)
		}
	}

	// Try /usr/share/wayland-sessions
	wSessions, err := d.discoverWaylandSessions()
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

func (d *Discoverer) discoverUserSessions(home string) ([]Session, error) {
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
				Path: path,
			})
		}
	}

	checkFile(".xinitrc", "Custom X Session (.xinitrc)")
	checkFile(".xsession", "Custom X Session (.xsession)")

	return sessions, nil
}

func (d *Discoverer) discoverCustomSessions(dir string, sessionType string) ([]Session, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []Session
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".desktop") {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				session, err := d.parseDesktopFile(filepath.Join(dir, name), sessionType)
				if err == nil {
					mu.Lock()
					sessions = append(sessions, session)
					mu.Unlock()
				}
			}(entry.Name())
		}
	}
	wg.Wait()
	return sessions, nil
}

func (d *Discoverer) discoverX11Sessions() ([]Session, error) {
	dir := X11SessionsDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sessions []Session
	for _, entry := range entries {
		if !entry.IsDir() {
			path := filepath.Join(dir, entry.Name())
			sessions = append(sessions, Session{
				Name: entry.Name(),
				Exec: path,
				Type: "X",
				Path: path,
			})
		}
	}
	return sessions, nil
}

func (d *Discoverer) discoverXSessions() ([]Session, error) {
	return d.discoverCustomSessions(XSessionsDir, "X")
}

func (d *Discoverer) discoverWaylandSessions() ([]Session, error) {
	return d.discoverCustomSessions(WaylandSessionsDir, "W")
}

// stripFreedesktopExecVariables removes Freedesktop Exec field codes from the command line.
// Some window managers or display managers specify %f, %u, etc., in their .desktop files.
// For a display manager, we generally want to remove these or ignore them.
func stripFreedesktopExecVariables(execCmd string) string {
	vars := []string{"%f", "%F", "%u", "%U", "%i", "%c", "%k"}
	for _, v := range vars {
		execCmd = strings.ReplaceAll(execCmd, v, "")
	}
	// Clean up any double spaces that might have been left behind
	execCmd = strings.Join(strings.Fields(execCmd), " ")
	return execCmd
}

func (d *Discoverer) parseDesktopFile(path string, defaultType string) (Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return Session{}, err
	}
	defer file.Close()

	var execCmd, name, tryExec string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch {
		case key == "Exec":
			execCmd = value
		case key == "TryExec":
			tryExec = value
		case key == "Name":
			name = value
		case strings.HasPrefix(key, "Name[") && strings.HasSuffix(key, "]") && name == "":
			name = value
		}
	}

	if execCmd == "" || name == "" {
		return Session{}, fmt.Errorf("missing Exec or Name")
	}

	execCmd = stripFreedesktopExecVariables(execCmd)

	if strings.Contains(execCmd, "startplasma-wayland") {
		// Use the SDDM plasma wrapper to ensure dbus and logind are correctly wired for KDE
		wrapper := PlasmaWaylandWrapperPath
		if _, err := d.ExecLookPath(wrapper); err == nil {
			execCmd = wrapper + " " + execCmd
		}
	}

	// Check TryExec if present
	if tryExec != "" {
		if _, err := d.ExecLookPath(tryExec); err != nil {
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
			if _, err := d.ExecLookPath(bin); err != nil {
				return Session{}, fmt.Errorf("executable not found: %s", bin)
			}
		}
	}

	return Session{
		Name: name,
		Exec: execCmd,
		Type: defaultType,
		Path: path,
	}, nil
}
