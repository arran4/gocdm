package cli

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/arran4/gocdm/auth"
	"github.com/arran4/gocdm/config"
	"github.com/arran4/gocdm/session"
	"github.com/arran4/gocdm/x11"
)

//go:embed testdata/graphical/*
var graphicalTestAssets embed.FS

type graphicalProbeReport struct {
	Arg0 string   `json:"arg0"`
	Args []string `json:"args"`
	Cwd  string   `json:"cwd"`
	Env  []string `json:"env"`
	UID  int      `json:"uid"`
	EUID int      `json:"euid"`
	GID  int      `json:"gid"`
	EGID int      `json:"egid"`
}

func buildGraphicalProbeBinaries(t *testing.T, binDir string, names ...string) {
	t.Helper()
	srcBin := filepath.Join(t.TempDir(), "graphicalprobe")
	cmd := exec.Command("go", "build", "-o", srcBin, "./testdata/graphicalprobe.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile graphicalprobe failed: %v\n%s", err, string(out))
	}
	payload, err := os.ReadFile(srcBin)
	if err != nil {
		t.Fatalf("read compiled graphicalprobe: %v", err)
	}
	for _, name := range names {
		dst := filepath.Join(binDir, name)
		if err := os.WriteFile(dst, payload, 0o755); err != nil {
			t.Fatalf("install probe binary %s: %v", dst, err)
		}
	}
}

func readGraphicalProbe(t *testing.T, probeDir, name string) graphicalProbeReport {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(probeDir, name+".json"))
	if err != nil {
		t.Fatalf("read probe output %s.json: %v", name, err)
	}
	var rep graphicalProbeReport
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("unmarshal probe output %s.json: %v", name, err)
	}
	return rep
}

func writeGraphicalTemplateDesktop(t *testing.T, targetPath, tmplName, binDir string) {
	t.Helper()
	raw, err := graphicalTestAssets.ReadFile("testdata/graphical/" + tmplName)
	if err != nil {
		t.Fatalf("read embedded template %s: %v", tmplName, err)
	}
	rendered := strings.ReplaceAll(string(raw), "__BIN_DIR__", binDir)
	if err := os.WriteFile(targetPath, []byte(rendered), 0o644); err != nil {
		t.Fatalf("write desktop %s: %v", targetPath, err)
	}
}

func plantDotfileSentinels(t *testing.T, homeDir string) {
	t.Helper()
	sentinel, err := graphicalTestAssets.ReadFile("testdata/graphical/dotfile_sentinel.sh")
	if err != nil {
		t.Fatalf("read embedded sentinel: %v", err)
	}
	for _, rc := range []string{".bashrc", ".bash_profile", ".profile", ".zshrc", ".zprofile"} {
		if err := os.WriteFile(filepath.Join(homeDir, rc), sentinel, 0o644); err != nil {
			t.Fatalf("write sentinel %s: %v", rc, err)
		}
	}
}

func TestParseGraphicalExecAndDesktopMetadata(t *testing.T) {
	parts, err := ParseGraphicalExec(`/usr/bin/startxfce4 --with-arg "two words" 'quoted$var'`)
	if err != nil {
		t.Fatalf("ParseGraphicalExec failed: %v", err)
	}
	want := []string{"/usr/bin/startxfce4", "--with-arg", "two words", "quoted$var"}
	if !reflect.DeepEqual(parts, want) {
		t.Fatalf("ParseGraphicalExec got %#v, want %#v", parts, want)
	}

	if _, err := ParseGraphicalExec(`"unterminated`); err == nil {
		t.Fatal("expected ParseGraphicalExec error on unterminated quote")
	}
	if _, err := ParseGraphicalExec("   \t "); err == nil {
		t.Fatal("expected ParseGraphicalExec error on empty command")
	}

	desktopSess, curDesktop := ResolveDesktopMetadata(session.Session{
		Name:         "Plasma (Wayland)",
		DesktopID:    "plasma",
		DesktopNames: []string{"KDE"},
	})
	if desktopSess != "plasma" || curDesktop != "KDE" {
		t.Fatalf("got (%q, %q), want (plasma, KDE)", desktopSess, curDesktop)
	}

	desktopSess, curDesktop = ResolveDesktopMetadata(session.Session{
		Name: "Custom Entry",
	})
	if desktopSess != "Custom Entry" || curDesktop != "Custom Entry" {
		t.Fatalf("fallback got (%q, %q), want (Custom Entry, Custom Entry)", desktopSess, curDesktop)
	}
}

func TestResolveUseConsoleKitPolicy(t *testing.T) {
	origLookPath := ExecLookPath
	t.Cleanup(func() { ExecLookPath = origLookPath })

	t.Run("default unset skips ConsoleKit when XDG_SESSION_ID present", func(t *testing.T) {
		ExecLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
		cfg := config.DefaultConfig()
		if ResolveUseConsoleKit(cfg, []string{"XDG_SESSION_ID=7"}) {
			t.Fatal("expected ConsoleKit to be skipped when XDG_SESSION_ID is active and consolekit is unset")
		}
	})

	t.Run("default unset uses ConsoleKit when XDG_SESSION_ID absent and ck-launch-session installed", func(t *testing.T) {
		ExecLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
		cfg := config.DefaultConfig()
		if !ResolveUseConsoleKit(cfg, []string{"HOME=/home/alice"}) {
			t.Fatal("expected ConsoleKit enabled when XDG_SESSION_ID absent and tool present")
		}
	})

	t.Run("default unset skips ConsoleKit when tool is missing", func(t *testing.T) {
		ExecLookPath = func(string) (string, error) { return "", os.ErrNotExist }
		cfg := config.DefaultConfig()
		if ResolveUseConsoleKit(cfg, []string{"HOME=/home/alice"}) {
			t.Fatal("expected ConsoleKit skipped when ck-launch-session missing and unset")
		}
	})

	t.Run("explicit consolekit=yes and consolekit=no are strictly honored", func(t *testing.T) {
		ExecLookPath = func(string) (string, error) { return "", os.ErrNotExist }
		yesCfg := &config.Config{ConsoleKit: true, ConsoleKitExplicit: true}
		if !ResolveUseConsoleKit(yesCfg, []string{"XDG_SESSION_ID=7"}) {
			t.Fatal("expected explicit consolekit=yes to be honored")
		}
		noCfg := &config.Config{ConsoleKit: false, ConsoleKitExplicit: true}
		if ResolveUseConsoleKit(noCfg, nil) {
			t.Fatal("expected explicit consolekit=no to be honored")
		}
	})
}

func setupIsolatedSessionDirs(t *testing.T) (x11Dir, xDir, wDir string) {
	t.Helper()
	base := t.TempDir()
	x11Dir = filepath.Join(base, "x11")
	xDir = filepath.Join(base, "xsessions")
	wDir = filepath.Join(base, "wayland-sessions")
	for _, d := range []string{x11Dir, xDir, wDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	origX11 := session.X11SessionsDir
	origXS := session.XSessionsDir
	origWS := session.WaylandSessionsDir
	origWrapper := session.PlasmaWaylandWrapperPath
	t.Cleanup(func() {
		session.X11SessionsDir = origX11
		session.XSessionsDir = origXS
		session.WaylandSessionsDir = origWS
		session.PlasmaWaylandWrapperPath = origWrapper
	})
	session.X11SessionsDir = x11Dir
	session.XSessionsDir = xDir
	session.WaylandSessionsDir = wDir
	return x11Dir, xDir, wDir
}

func TestGettyPlasmaWaylandSessionE2E(t *testing.T) {
	setupGettyContextTestEnv(t)
	origStat := osStat
	t.Cleanup(func() { osStat = origStat })

	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(tmpDir, "bin")
	aliceHome := filepath.Join(tmpDir, "alice")
	probeDir := filepath.Join(tmpDir, "probes")
	for _, d := range []string{binDir, aliceHome, probeDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	buildGraphicalProbeBinaries(t, binDir, "plasma-dbus-run-session-if-needed", "startplasma-wayland")
	plantDotfileSentinels(t, aliceHome)

	_, _, waylandDir := setupIsolatedSessionDirs(t)
	session.PlasmaWaylandWrapperPath = filepath.Join(binDir, "plasma-dbus-run-session-if-needed")
	writeGraphicalTemplateDesktop(t, filepath.Join(waylandDir, "plasma.desktop"), "plasma.desktop.tmpl", binDir)

	DiscoverSessionsFn = func(userHome string) ([]session.Session, error) {
		d := session.NewDiscoverer()
		d.ShellsFile = os.DevNull
		d.OpenShells = func(string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("")), nil
		}
		return d.Discover(userHome)
	}

	uid := os.Geteuid()
	passwdPath := filepath.Join(tmpDir, "passwd")
	if err := os.WriteFile(passwdPath, []byte(fmt.Sprintf("alice:x:%d:%d:Alice:%s:/bin/zsh\n", uid, uid, aliceHome)), 0o644); err != nil {
		t.Fatal(err)
	}
	pamEnvPath := filepath.Join(tmpDir, "pam_env.conf")
	if err := os.WriteFile(pamEnvPath, []byte("GOCDM_PROBE_DIR OVERRIDE="+probeDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	PasswdFilePath = passwdPath
	PamEnvConfPath = pamEnvPath

	emptyCfg := filepath.Join(tmpDir, "cdmrc")
	if err := os.WriteFile(emptyCfg, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	osReadlink = func(p string) (string, error) {
		if p == "/proc/self/fd/0" {
			return "/dev/tty2", nil
		}
		return "", os.ErrNotExist
	}
	osStat = func(p string) (os.FileInfo, error) {
		if p == "/dev/tty2" {
			return mockFileInfo{mode: os.ModeCharDevice}, nil
		}
		return origStat(p)
	}

	TuiPromptCredentials = func(string, string, string, func(string, string) error) (string, string, error) {
		return "alice", "pw", nil
	}
	SaveStateAsUserFn = func(acct *config.AccountInfo, lastSession string) error {
		return config.SafeSaveStateAt(acct.HomeDir, lastSession, acct.UID)
	}
	t.Setenv("AWS_SECRET_ACCESS_KEY", "must-not-leak")
	t.Setenv("LD_PRELOAD", "/tmp/evil.so")
	t.Setenv("GOCDM_PROBE_EXIT", "")
	t.Setenv("GOCDM_PROBE_FAIL_CMD", "")

	t.Run("inherits PAM runtime dbus metadata cwd and avoids shell dotfiles", func(t *testing.T) {
		var events []string
		var closed bool
		sess := &spyLoginSession{
			username:    "alice",
			events:      &events,
			closeCalled: &closed,
			pamEnv: []string{
				"XDG_RUNTIME_DIR=/run/user/1001",
				"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus",
				"XDG_SESSION_ID=11",
			},
		}
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{events: &events, session: sess}
		}
		TuiPromptCredentials = func(_ string, _ string, _ string, authFn func(string, string) error) (string, string, error) {
			return "alice", "pw", authFn("alice", "pw")
		}
		PrepareTTYFn = func(*config.AccountInfo) (TTYCleanup, string, error) {
			events = append(events, "prepare_tty")
			return func() error {
				events = append(events, "restore_tty")
				return nil
			}, "2", nil
		}

		var exited bool
		var code int
		Run([]string{"-login", "-config", emptyCfg}, func(c int) { exited = true; code = c })
		if !exited || code != 0 || !closed {
			t.Fatalf("Plasma Wayland launch failed: exited=%v code=%d closed=%v", exited, code, closed)
		}

		if sess.pamPutEnv["XDG_SESSION_TYPE"] != "wayland" ||
			sess.pamPutEnv["XDG_SESSION_CLASS"] != "user" ||
			sess.pamPutEnv["XDG_SESSION_DESKTOP"] != "plasma" ||
			sess.pamPutEnv["XDG_CURRENT_DESKTOP"] != "KDE" ||
			sess.pamPutEnv["XDG_VTNR"] != "2" ||
			sess.pamPutEnv["XDG_SEAT"] != "seat0" {
			t.Fatalf("unexpected PAM metadata passed to OpenSession: %+v", sess.pamPutEnv)
		}

		wrapperRep := readGraphicalProbe(t, probeDir, "plasma-dbus-run-session-if-needed")
		compRep := readGraphicalProbe(t, probeDir, "startplasma-wayland")

		if wrapperRep.Cwd != aliceHome || compRep.Cwd != aliceHome {
			t.Fatalf("unexpected cwd: wrapper=%q compositor=%q, want %q", wrapperRep.Cwd, compRep.Cwd, aliceHome)
		}
		if len(wrapperRep.Args) != 2 || wrapperRep.Args[1] != filepath.Join(binDir, "startplasma-wayland") {
			t.Fatalf("unexpected wrapper argv: %#v", wrapperRep.Args)
		}
		if len(compRep.Args) != 1 {
			t.Fatalf("unexpected compositor argv: %#v", compRep.Args)
		}

		envMap := config.EnvSliceToMap(compRep.Env)
		if envMap["USER"] != "alice" || envMap["LOGNAME"] != "alice" || envMap["HOME"] != aliceHome || envMap["SHELL"] != "/bin/zsh" {
			t.Fatalf("unexpected identity env: %v", envMap)
		}
		if envMap["PATH"] != "/usr/local/bin:/usr/bin:/bin" {
			t.Fatalf("unexpected PATH=%q", envMap["PATH"])
		}
		if envMap["XDG_SESSION_TYPE"] != "wayland" ||
			envMap["XDG_SESSION_CLASS"] != "user" ||
			envMap["XDG_SESSION_DESKTOP"] != "plasma" ||
			envMap["DESKTOP_SESSION"] != "plasma" ||
			envMap["XDG_CURRENT_DESKTOP"] != "KDE" ||
			envMap["XDG_RUNTIME_DIR"] != "/run/user/1001" ||
			envMap["DBUS_SESSION_BUS_ADDRESS"] != "unix:path=/run/user/1001/bus" ||
			envMap["XDG_SESSION_ID"] != "11" {
			t.Fatalf("unexpected Plasma Wayland XDG/D-Bus env: %v", envMap)
		}
		for _, leaked := range []string{"AWS_SECRET_ACCESS_KEY", "LD_PRELOAD", "DOTFILE_SOURCED"} {
			if _, ok := envMap[leaked]; ok {
				t.Fatalf("unexpected leaked/sourced var %s=%q", leaked, envMap[leaked])
			}
		}
		if _, err := os.Stat(filepath.Join(aliceHome, ".dotfile_sourced_marker")); !os.IsNotExist(err) {
			t.Fatalf("interactive shell dotfiles must not be sourced for graphical Wayland launch")
		}
	})

	t.Run("does not fabricate XDG_RUNTIME_DIR or DBUS_SESSION_BUS_ADDRESS when PAM omits them and recovers cleanly for next login", func(t *testing.T) {
		if err := os.WriteFile(pamEnvPath, []byte("GOCDM_PROBE_DIR OVERRIDE="+probeDir+"\nGOCDM_PROBE_FAIL_CMD OVERRIDE=startplasma-wayland\nGOCDM_PROBE_EXIT OVERRIDE=23\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = os.WriteFile(pamEnvPath, []byte("GOCDM_PROBE_DIR OVERRIDE="+probeDir+"\n"), 0o644)
		})

		var events []string
		var closed bool
		sess := &spyLoginSession{
			username:    "alice",
			events:      &events,
			closeCalled: &closed,
			pamEnv:      nil,
		}
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{events: &events, session: sess}
		}
		PrepareTTYFn = func(*config.AccountInfo) (TTYCleanup, string, error) {
			events = append(events, "prepare_tty")
			return func() error {
				events = append(events, "restore_tty")
				return nil
			}, "2", nil
		}

		var exited bool
		var code int
		Run([]string{"-login", "-config", emptyCfg}, func(c int) { exited = true; code = c })
		if !exited || code != 23 || !closed {
			t.Fatalf("expected exit(23) and PAM closed=true, got exited=%v code=%d closed=%v", exited, code, closed)
		}
		if len(events) < 2 || events[len(events)-2] != "restore_tty" || events[len(events)-1] != "close_session" {
			t.Fatalf("expected restore_tty then close_session at teardown, got %v", events)
		}

		compRep := readGraphicalProbe(t, probeDir, "startplasma-wayland")
		envMap := config.EnvSliceToMap(compRep.Env)
		for _, forbidden := range []string{"XDG_RUNTIME_DIR", "DBUS_SESSION_BUS_ADDRESS", "WAYLAND_DISPLAY", "XAUTHORITY"} {
			if _, ok := envMap[forbidden]; ok {
				t.Fatalf("must not fabricate %s when PAM omitted it, got %q", forbidden, envMap[forbidden])
			}
		}

		// Subsequent sequential login after failure must still authenticate cleanly with a new PAM session.
		_ = os.WriteFile(pamEnvPath, []byte("GOCDM_PROBE_DIR OVERRIDE="+probeDir+"\n"), 0o644)
		var closed2 bool
		sess2 := &spyLoginSession{username: "alice", closeCalled: &closed2}
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{session: sess2}
		}
		exited = false
		code = -1
		Run([]string{"-login", "-config", emptyCfg}, func(c int) { exited = true; code = c })
		if !exited || code != 0 || !closed2 {
			t.Fatalf("subsequent login failed: exited=%v code=%d closed=%v", exited, code, closed2)
		}
	})
}

func TestGettyAndUserX11GraphicalSession(t *testing.T) {
	setupGettyContextTestEnv(t)
	origStat := osStat
	origX11Exec := x11.ExecCommand
	origX11SocketDir := x11.X11SocketDir
	origLaunchX := LaunchXSessionFn
	t.Cleanup(func() {
		osStat = origStat
		x11.ExecCommand = origX11Exec
		x11.X11SocketDir = origX11SocketDir
		LaunchXSessionFn = origLaunchX
	})
	x11.ExecCommand = exec.Command
	x11.X11SocketDir = t.TempDir()

	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(tmpDir, "bin")
	aliceHome := filepath.Join(tmpDir, "alice")
	probeDir := filepath.Join(tmpDir, "probes")
	for _, d := range []string{binDir, aliceHome, probeDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	buildGraphicalProbeBinaries(t, binDir, "startx", "chvt", "startxfce4", "plasma-dbus-run-session-if-needed", "startplasma-wayland")
	plantDotfileSentinels(t, aliceHome)

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOCDM_PROBE_DIR", probeDir)

	_, xsessionsDir, _ := setupIsolatedSessionDirs(t)
	writeGraphicalTemplateDesktop(t, filepath.Join(xsessionsDir, "plasmax11.desktop"), "x11_quoted.desktop.tmpl", binDir)

	DiscoverSessionsFn = func(userHome string) ([]session.Session, error) {
		d := session.NewDiscoverer()
		d.ShellsFile = os.DevNull
		d.OpenShells = func(string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("")), nil
		}
		return d.Discover(userHome)
	}

	uid := os.Geteuid()
	passwdPath := filepath.Join(tmpDir, "passwd")
	if err := os.WriteFile(passwdPath, []byte(fmt.Sprintf("alice:x:%d:%d:Alice:%s:/bin/bash\n", uid, uid, aliceHome)), 0o644); err != nil {
		t.Fatal(err)
	}
	pamEnvPath := filepath.Join(tmpDir, "pam_env.conf")
	if err := os.WriteFile(pamEnvPath, []byte("GOCDM_PROBE_DIR OVERRIDE="+probeDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	PasswdFilePath = passwdPath
	PamEnvConfPath = pamEnvPath

	cfgPath := filepath.Join(tmpDir, "cdmrc")
	if err := os.WriteFile(cfgPath, []byte("xtty=7\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	osReadlink = func(p string) (string, error) {
		if p == "/proc/self/fd/0" {
			return "/dev/tty2", nil
		}
		return "", os.ErrNotExist
	}
	osStat = func(p string) (os.FileInfo, error) {
		if p == "/dev/tty2" {
			return mockFileInfo{mode: os.ModeCharDevice}, nil
		}
		return origStat(p)
	}

	SaveStateAsUserFn = func(acct *config.AccountInfo, lastSession string) error {
		return config.SafeSaveStateAt(acct.HomeDir, lastSession, acct.UID)
	}

	t.Run("getty X11 preserves quoted args sets DesktopNames skips ck-launch-session with logind and cleans up", func(t *testing.T) {
		var events []string
		var closed bool
		sess := &spyLoginSession{
			username:    "alice",
			events:      &events,
			closeCalled: &closed,
			pamEnv: []string{
				"XDG_RUNTIME_DIR=/run/user/1001",
				"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus",
				"XDG_SESSION_ID=19",
			},
		}
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{events: &events, session: sess}
		}
		TuiPromptCredentials = func(_ string, _ string, _ string, authFn func(string, string) error) (string, string, error) {
			return "alice", "pw", authFn("alice", "pw")
		}
		PrepareTTYFn = func(*config.AccountInfo) (TTYCleanup, string, error) {
			events = append(events, "prepare_tty")
			return func() error {
				events = append(events, "restore_tty")
				return nil
			}, "2", nil
		}
		LaunchSupervisedXSessionFn = x11.LaunchXSessionWithOptions

		var exited bool
		var code int
		Run([]string{"-login", "-config", cfgPath}, func(c int) { exited = true; code = c })
		if !exited || code != 0 || !closed {
			t.Fatalf("X11 getty launch failed: exited=%v code=%d closed=%v", exited, code, closed)
		}

		if sess.pamPutEnv["XDG_SESSION_TYPE"] != "x11" ||
			sess.pamPutEnv["XDG_SESSION_DESKTOP"] != "plasmax11" ||
			sess.pamPutEnv["XDG_CURRENT_DESKTOP"] != "XFCE:GTK" ||
			sess.pamPutEnv["XDG_VTNR"] != "7" {
			t.Fatalf("unexpected PAM X11 metadata: %+v", sess.pamPutEnv)
		}

		startxRep := readGraphicalProbe(t, probeDir, "startx")
		if startxRep.Cwd != aliceHome {
			t.Fatalf("startx cwd=%q, want %q", startxRep.Cwd, aliceHome)
		}
		wantArgs := []string{
			"startx",
			filepath.Join(binDir, "startxfce4"),
			"--with-arg",
			"two words",
			"--",
			":0",
			"-nolisten",
			"tcp",
			"vt7",
		}
		if !reflect.DeepEqual(startxRep.Args, wantArgs) {
			t.Fatalf("startx argv=%#v, want %#v (quoted arg lost or ck-launch-session unexpectedly added)", startxRep.Args, wantArgs)
		}
		envMap := config.EnvSliceToMap(startxRep.Env)
		if envMap["XDG_SESSION_TYPE"] != "x11" ||
			envMap["XDG_SESSION_DESKTOP"] != "plasmax11" ||
			envMap["DESKTOP_SESSION"] != "plasmax11" ||
			envMap["XDG_CURRENT_DESKTOP"] != "XFCE:GTK" {
			t.Fatalf("unexpected X11 session env: %v", envMap)
		}
		if _, err := os.Stat(filepath.Join(aliceHome, ".dotfile_sourced_marker")); !os.IsNotExist(err) {
			t.Fatal("interactive shell dotfiles must not be sourced for graphical X11 launch")
		}
		chvtRep := readGraphicalProbe(t, probeDir, "chvt")
		if len(chvtRep.Args) != 2 || chvtRep.Args[1] != "7" {
			t.Fatalf("unexpected chvt args: %#v", chvtRep.Args)
		}
	})

	t.Run("X11 child failure still restores TTY and closes PAM session", func(t *testing.T) {
		if err := os.WriteFile(pamEnvPath, []byte("GOCDM_PROBE_DIR OVERRIDE="+probeDir+"\nGOCDM_PROBE_FAIL_CMD OVERRIDE=startx\nGOCDM_PROBE_EXIT OVERRIDE=1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = os.WriteFile(pamEnvPath, []byte("GOCDM_PROBE_DIR OVERRIDE="+probeDir+"\n"), 0o644)
		})

		var events []string
		var closed bool
		sess := &spyLoginSession{username: "alice", events: &events, closeCalled: &closed}
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{events: &events, session: sess}
		}
		PrepareTTYFn = func(*config.AccountInfo) (TTYCleanup, string, error) {
			return func() error {
				events = append(events, "restore_tty")
				return nil
			}, "2", nil
		}
		LaunchSupervisedXSessionFn = x11.LaunchXSessionWithOptions

		var exited bool
		var code int
		Run([]string{"-login", "-config", cfgPath}, func(c int) { exited = true; code = c })
		if !exited || code != 1 || !closed {
			t.Fatalf("expected X11 failure exit(1) and PAM closed=true, got exited=%v code=%d closed=%v", exited, code, closed)
		}
		if len(events) < 2 || events[len(events)-2] != "restore_tty" || events[len(events)-1] != "close_session" {
			t.Fatalf("expected restore_tty then close_session on X11 failure, got %v", events)
		}
	})

	t.Run("skips chvt when target VT matches CurrentVT", func(t *testing.T) {
		sameVTCfg := filepath.Join(tmpDir, "samevt-cdmrc")
		if err := os.WriteFile(sameVTCfg, []byte("xtty=2\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = os.Remove(filepath.Join(probeDir, "chvt.json"))
		_ = os.Remove(filepath.Join(probeDir, "startx.json"))

		var closed bool
		sess := &spyLoginSession{
			username:    "alice",
			closeCalled: &closed,
			pamEnv:      []string{"XDG_SESSION_ID=20"},
		}
		NewAuthenticator = func(string) auth.Authenticator {
			return spyAuthenticator{session: sess}
		}
		LaunchSupervisedXSessionFn = x11.LaunchXSessionWithOptions

		var exited bool
		var code int
		Run([]string{"-login", "-config", sameVTCfg}, func(c int) { exited = true; code = c })
		if !exited || code != 0 || !closed {
			t.Fatalf("expected clean exit on same-VT X11 launch, got exited=%v code=%d closed=%v", exited, code, closed)
		}
		if _, err := os.Stat(filepath.Join(probeDir, "chvt.json")); !os.IsNotExist(err) {
			t.Fatalf("did not expect chvt to execute when CurrentVT == VT (2)")
		}
		startxRep := readGraphicalProbe(t, probeDir, "startx")
		if len(startxRep.Args) == 0 || startxRep.Args[len(startxRep.Args)-1] != "vt2" {
			t.Fatalf("expected startx args ending in vt2, got %#v", startxRep.Args)
		}
	})

	t.Run("post-login ModeUserSession preserves user env and launches X11 and Wayland without PAM prompt", func(t *testing.T) {
		t.Setenv("HOME", aliceHome)
		t.Setenv("XDG_RUNTIME_DIR", "/run/user/1001")
		t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/run/user/1001/bus")
		t.Setenv("XDG_SESSION_ID", "42")
		osGetuid = func() int { return uid }
		osGeteuid = func() int { return uid }

		NewAuthenticator = func(string) auth.Authenticator {
			t.Fatal("user session mode must never invoke PAM authenticator")
			return nil
		}
		TuiPromptCredentials = func(string, string, string, func(string, string) error) (string, string, error) {
			t.Fatal("user session mode must never prompt for credentials")
			return "", "", nil
		}

		// First test X11 autostart
		var capturedBin []string
		var capturedCK bool
		var capturedEnv []string
		LaunchXSessionFn = func(bin []string, display int, vt string, consoleKit bool, ckTimeout int, altStartX bool, startXLog string, serverArgs []string, env []string) error {
			capturedBin = bin
			capturedCK = consoleKit
			capturedEnv = env
			return nil
		}

		var exited bool
		Run([]string{"-mode=user", "-config", cfgPath}, func(int) { exited = true })
		if exited {
			t.Fatal("did not expect non-zero exit in user mode X11 launch")
		}
		if capturedCK {
			t.Fatal("expected ConsoleKit disabled in user mode when XDG_SESSION_ID is active")
		}
		if len(capturedBin) != 3 || capturedBin[1] != "--with-arg" || capturedBin[2] != "two words" {
			t.Fatalf("unexpected user-mode X11 argv: %#v", capturedBin)
		}
		envMap := config.EnvSliceToMap(capturedEnv)
		if envMap["XDG_RUNTIME_DIR"] != "/run/user/1001" ||
			envMap["DBUS_SESSION_BUS_ADDRESS"] != "unix:path=/run/user/1001/bus" ||
			envMap["XDG_SESSION_DESKTOP"] != "plasmax11" ||
			envMap["XDG_CURRENT_DESKTOP"] != "XFCE:GTK" {
			t.Fatalf("unexpected user-mode X11 env: %v", envMap)
		}

		// Now switch discovered sessions to Wayland and test ModeUserSession
		_ = os.Remove(filepath.Join(xsessionsDir, "plasmax11.desktop"))
		_, _, waylandDir := setupIsolatedSessionDirs(t)
		session.PlasmaWaylandWrapperPath = filepath.Join(binDir, "plasma-dbus-run-session-if-needed")
		writeGraphicalTemplateDesktop(t, filepath.Join(waylandDir, "plasma.desktop"), "plasma.desktop.tmpl", binDir)

		var waylandBinary string
		var waylandArgs []string
		var waylandEnv []string
		ExecProgramFn = func(binary string, argv []string, env []string) error {
			waylandBinary = binary
			waylandArgs = argv
			waylandEnv = env
			return nil
		}
		Run([]string{"-mode=user", "-config", cfgPath}, func(int) { exited = true })
		if waylandBinary != filepath.Join(binDir, "plasma-dbus-run-session-if-needed") {
			t.Fatalf("unexpected user-mode Wayland binary %q, args=%#v", waylandBinary, waylandArgs)
		}
		wEnvMap := config.EnvSliceToMap(waylandEnv)
		if wEnvMap["XDG_SESSION_TYPE"] != "wayland" || wEnvMap["XDG_SESSION_DESKTOP"] != "plasma" || wEnvMap["XDG_CURRENT_DESKTOP"] != "KDE" {
			t.Fatalf("unexpected user-mode Wayland env: %v", wEnvMap)
		}
	})
}
