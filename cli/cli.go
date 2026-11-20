package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/arran4/gocdm/auth"
	"github.com/arran4/gocdm/config"
	"github.com/arran4/gocdm/dialog"
	"github.com/arran4/gocdm/session"
	"github.com/arran4/gocdm/x11"
	"golang.org/x/term"
)

var Version = "dev"
var IsTerminal = term.IsTerminal
var NewAuthenticator = func(service string) auth.Authenticator { return auth.NewPAMAuthenticator(service) }
var PromptCredentials = auth.PromptCredentials
var TuiPromptCredentials = func(title, theme, version string, authFunc func(string, string) error) (string, string, error) {
	return dialog.ShowLogin(title, theme, version, authFunc)
}
var ExecLookPath = exec.LookPath
var DropPrivilegesFn = DropPrivileges
var ExecProgramFn = ExecProgram
var LaunchXSessionFn = x11.LaunchXSession
var LaunchSupervisedXSessionFn = x11.LaunchXSessionWithOptions
var DiscoverSessionsFn = session.DiscoverSessions
var LookupAccountFn = config.LookupAccount
var ChdirFn = os.Chdir
var SaveStateFn = config.SafeSaveStateAt
var SaveStateAsUserFn = saveStateAsUser
var PrepareTTYFn = prepareTTYForAccount
var RunSupervisedSessionFn = runSupervisedSession
var NoticesStdout io.Writer = os.Stdout
var NoticesStderr io.Writer = os.Stderr
var osStat = os.Stat
var osReadlink = os.Readlink

var SupportsLoginModeFn = supportsLoginMode
var SupportsXSessionsFn = supportsXSessions
var PlatformSupportTierFn = platformSupportTier

var PasswdFilePath = "/etc/passwd"
var PamEnvConfPath = "/etc/security/pam_env.conf"

var debugEnvAllowlist = map[string]bool{
	"DBUS_SESSION_BUS_ADDRESS": true,
	"DESKTOP_SESSION":          true,
	"HOME":                     true,
	"LANG":                     true,
	"LOGNAME":                  true,
	"PATH":                     true,
	"PWD":                      true,
	"SHELL":                    true,
	"TERM":                     true,
	"USER":                     true,
	"XDG_CURRENT_DESKTOP":      true,
	"XDG_RUNTIME_DIR":          true,
	"XDG_SEAT":                 true,
	"XDG_SESSION_CLASS":        true,
	"XDG_SESSION_DESKTOP":      true,
	"XDG_SESSION_ID":           true,
	"XDG_SESSION_TYPE":         true,
	"XDG_VTNR":                 true,
}

func runInternalSaveState(exit func(int)) {
	if osGeteuid() == 0 {
		fmt.Fprintln(os.Stderr, "refusing internal state write as root")
		exit(1)
		return
	}
	home := os.Getenv("GOCDM_STATE_HOME")
	sessionName := os.Getenv("GOCDM_STATE_SESSION")
	uidStr := os.Getenv("GOCDM_STATE_UID")
	uid, err := strconv.Atoi(uidStr)
	if err != nil || uid <= 0 {
		fmt.Fprintf(os.Stderr, "invalid internal state uid %q\n", uidStr)
		exit(1)
		return
	}
	if err := SaveStateFn(home, sessionName, uid); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save state: %v\n", err)
		exit(1)
		return
	}
	exit(0)
}

func Run(args []string, exit func(int)) {
	if os.Getenv("GOCDM_INTERNAL_SAVE_STATE") == "1" {
		runInternalSaveState(exit)
		return
	}

	fs := flag.NewFlagSet("gocdm", flag.ContinueOnError)
	configPathFlag := fs.String("config", "", "Path to config file")
	dryRun := fs.Bool("dry-run", false, "Dry run mode (print command instead of executing)")
	forceMenu := fs.Bool("menu", false, "Force menu display even if only one session is found")
	showVersion := fs.Bool("version", false, "Show version information")
	loginMode := fs.Bool("login", false, "Prompt for username/password and authenticate with PAM")
	modeFlag := fs.String("mode", string(ModeAuto), "Operating mode: auto, login (getty), or user (autostart)")
	tuiLogin := fs.Bool("tui-login", true, "Use TUI for login prompt (when -login is enabled)")
	pamService := fs.String("pam-service", "login", "PAM service name used with -login")
	debugEnv := fs.Bool("debug-env", false, "Print session environment before exec (redacts sensitive values)")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			exit(0)
		} else {
			exit(2)
		}
		return
	}

	if *showVersion {
		fmt.Printf("gocdm version %s\n", Version)
		exit(0)
		return
	}

	var modeFlagSet, loginFlagSet bool
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "mode":
			modeFlagSet = true
		case "login":
			loginFlagSet = true
		}
	})

	modeSel, err := ResolveModeSelection(modeFlagSet, *modeFlag, loginFlagSet, *loginMode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		exit(2)
		return
	}

	var configPath string
	if *configPathFlag != "" && fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "cannot use both positional config path and -config; use one source")
		exit(2)
		return
	}

	opMode, _, err := ResolveOperatingMode(modeSel, defaultProcessContext(), *dryRun)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		exit(1)
		return
	}
	isLoginMode := opMode == ModeGettyLogin

	if *configPathFlag != "" {
		configPath = *configPathFlag
	} else if fs.NArg() > 0 {
		// Backward compatibility: first positional argument is config path
		configPath = fs.Arg(0)
	}

	var cfg *config.Config
	if isLoginMode {
		cfg, err = config.LoadSystemConfig(configPath)
	} else {
		cfg, err = config.LoadConfig(configPath)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		exit(1)
		return
	}

	if err := validateTTY(*dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		exit(1)
		return
	}

	var (
		acct         *config.AccountInfo
		loginSession auth.LoginSession
		state        = &config.State{}
		userHome     string
	)

	if isLoginMode {
		authenticator := NewAuthenticator(*pamService)
		var promptedUser, password string

		authWrapper := func(user, pass string) error {
			ls, authErr := authenticator.Authenticate(user, pass)
			if authErr == nil {
				loginSession = ls
			}
			return authErr
		}

		if *tuiLogin {
			var tuiErr error
			promptedUser, password, tuiErr = TuiPromptCredentials("Console Display Manager - Login", cfg.DialogRC, Version, authWrapper)
			_ = password
			if tuiErr != nil {
				fmt.Fprintf(os.Stderr, "Authentication prompt failed or cancelled: %v\n", tuiErr)
				exit(1)
				return
			}
		} else {
			for {
				promptedUser, password, err = PromptCredentials(os.Stdin, os.Stdout)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Authentication prompt failed: %v\n", err)
					exit(1)
					return
				}
				if err := authWrapper(promptedUser, password); err == nil {
					break
				} else {
					fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
				}
			}
		}

		if loginSession != nil && loginSession.Username() != "" {
			promptedUser = loginSession.Username()
		}

		acct, err = LookupAccountFn(promptedUser, PasswdFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resolve authenticated account %q: %v\n", promptedUser, err)
			closeLoginSession(loginSession)
			exit(1)
			return
		}

		userHome = acct.HomeDir
		privileged := osGeteuid() == 0 && acct.UID != 0
		cfg, err = config.LoadConfigForUser(configPath, acct.HomeDir, acct.UID, privileged)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			closeLoginSession(loginSession)
			exit(1)
			return
		}

		if config.ValidateHomeDir(acct.HomeDir) == nil {
			loadedState, stateErr := config.LoadStateForUser(acct.HomeDir, acct.UID)
			if stateErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to load saved session state: %v\n", stateErr)
			} else {
				state = loadedState
			}
		}
	} else {
		userHome, _ = os.UserHomeDir()
		if userHome != "" {
			if loadedState, stateErr := config.LoadStateAt(userHome); stateErr == nil {
				state = loadedState
			}
		}
	}

	discoverHome := userHome
	if isLoginMode && config.ValidateHomeDir(discoverHome) != nil {
		discoverHome = ""
	}
	sessions, err := resolveSessions(cfg, discoverHome)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		closeLoginSession(loginSession)
		exit(1)
		return
	}

	var selectedIdx int
	// Show menu if more than 1 session OR forceMenu is true
	if len(sessions) == 1 && !*forceMenu {
		selectedIdx = 0
	} else {
		optionNames := make([]string, len(sessions))
		details := make([]string, len(sessions))
		defaultIdx := 0
		for i, s := range sessions {
			optionNames[i] = s.Name
			details[i] = fmt.Sprintf("Name: %s\nExec: %s\nType: %s\nPath: %s", s.Name, s.Exec, s.Type, s.Path)
			if s.Name == state.LastSession {
				defaultIdx = i
			}
		}

		idx, err := dialog.ShowMenu("Console Display Manager", optionNames, details, cfg.CountFrom, defaultIdx, cfg.DialogRC, Version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Selection cancelled or error: %v\n", err)
			closeLoginSession(loginSession)
			exit(2)
			return
		}
		selectedIdx = idx
	}

	if selectedIdx < 0 || selectedIdx >= len(sessions) {
		fmt.Fprintf(os.Stderr, "Invalid selection index: %d\n", selectedIdx)
		closeLoginSession(loginSession)
		exit(1)
		return
	}

	selectedSession := sessions[selectedIdx]

	prepareGettySession := func(stype string) ([]string, string, TTYCleanup, bool) {
		if loginSession != nil {
			setPAMSessionMetadata(loginSession, stype, selectedSession.Name, detectCurrentVT())
			if err := loginSession.OpenSession(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: PAM open session failed: %v\n", err)
				closeLoginSession(loginSession)
				exit(1)
				return nil, "", noopTTYCleanup, false
			}
			auth.FlushNotices(loginSession.DrainNotices(), NoticesStdout, NoticesStderr)
		}

		sessionCwd, cwdErr := resolveSessionCwd(acct.HomeDir)
		if cwdErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to set working directory: %v\n", cwdErr)
			closeLoginSession(loginSession)
			exit(1)
			return nil, "", noopTTYCleanup, false
		}

		var pamEnv []string
		if loginSession != nil {
			pamEnv = loginSession.Env()
		}
		env, envErr := config.BuildGettySessionEnv(config.GettyEnvOptions{
			LauncherEnv:   os.Environ(),
			Account:       acct,
			PamEnvFile:    PamEnvConfPath,
			PamSessionEnv: pamEnv,
			SessionType:   stype,
			SessionName:   selectedSession.Name,
			WorkingDir:    sessionCwd,
			SpawnPID:      os.Getpid(),
		})
		if envErr != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to build secure session environment: %v\n", envErr)
			closeLoginSession(loginSession)
			exit(1)
			return nil, "", noopTTYCleanup, false
		}

		ttyCleanup, _, ttyErr := PrepareTTYFn(acct)
		if ttyErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to prepare TTY for user: %v\n", ttyErr)
			closeLoginSession(loginSession)
			exit(1)
			return nil, "", noopTTYCleanup, false
		}
		if ttyCleanup == nil {
			ttyCleanup = noopTTYCleanup
		}

		if err := SaveStateAsUserFn(acct, selectedSession.Name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to save state: %v\n", err)
		}

		if *debugEnv {
			fmt.Print(formatDebugEnv(env))
		}
		return env, sessionCwd, ttyCleanup, true
	}

	prepareUserSession := func(stype string) []string {
		env := config.BuildUserSessionEnv(config.UserEnvOptions{
			BaseEnv:     os.Environ(),
			SessionType: stype,
			SessionName: selectedSession.Name,
			SpawnPID:    os.Getpid(),
		})
		if userHome != "" {
			if err := SaveStateFn(userHome, selectedSession.Name, osGeteuid()); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to save state: %v\n", err)
			}
		}
		if *debugEnv {
			fmt.Print(formatDebugEnv(env))
		}
		return env
	}

	flagVal := normalizeSessionType(selectedSession.Type)

	switch flagVal {
	case "W", "C":
		parts := strings.Fields(selectedSession.Exec)
		if len(parts) == 0 {
			fmt.Fprintln(os.Stderr, "Empty command")
			closeLoginSession(loginSession)
			exit(1)
			return
		}
		bin := parts[0]
		cmdArgs := parts[1:]

		label := "Wayland"
		if flagVal == "C" {
			label = "console"
		}
		if *dryRun {
			fmt.Printf("Dry run: would execute %s program: %s %v\n", label, bin, cmdArgs)
			closeLoginSession(loginSession)
			return
		}

		binary, err := ExecLookPath(bin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Command not found: %s\n", bin)
			closeLoginSession(loginSession)
			exit(1)
			return
		}

		if isLoginMode {
			env, sessionCwd, ttyCleanup, ok := prepareGettySession(flagVal)
			if !ok {
				return
			}
			code, runErr := RunSupervisedSessionFn(SupervisedSessionSpec{
				Binary:  binary,
				Args:    append([]string{bin}, cmdArgs...),
				Env:     env,
				Dir:     sessionCwd,
				Account: acct,
				Stdin:   os.Stdin,
				Stdout:  os.Stdout,
				Stderr:  os.Stderr,
			})
			if ttyErr := ttyCleanup(); ttyErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to restore TTY ownership: %v\n", ttyErr)
			}
			closeLoginSession(loginSession)
			if runErr != nil {
				fmt.Fprintf(os.Stderr, "Failed to run session: %v\n", runErr)
				exit(1)
				return
			}
			exit(code)
			return
		}

		env := prepareUserSession(flagVal)
		if err := ExecProgramFn(binary, append([]string{bin}, cmdArgs...), env); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to exec: %v\n", err)
			exit(1)
			return
		}

	case "X":
		if !SupportsXSessionsFn() {
			fmt.Fprintf(os.Stderr, "X session launch is not supported on %s (support tier: %s)\n", runtime.GOOS, PlatformSupportTierFn())
			closeLoginSession(loginSession)
			exit(1)
			return
		}

		// If X is already running and locktty=yes, activate it.
		if cfg.LockTTY && x11.IsDisplayActive(cfg.Display) {
			vt, err := x11.GetVT(cfg.XTTY, cfg.Display)
			if err != nil {
				if *dryRun {
					fmt.Fprintf(os.Stderr, "Dry run warning: Failed to get VT for locktty: %v. Assuming VT7.\n", err)
					vt = "7"
				} else {
					fmt.Fprintf(os.Stderr, "Failed to get VT for locktty: %v\n", err)
					closeLoginSession(loginSession)
					exit(1)
					return
				}
			}

			if *dryRun {
				fmt.Printf("Dry run: would switch to existing X session on display :%d VT%s\n", cfg.Display, vt)
				closeLoginSession(loginSession)
				return
			}

			if err := x11.SwitchVT(vt); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to switch VT: %v\n", err)
				closeLoginSession(loginSession)
				exit(1)
				return
			}

			closeLoginSession(loginSession)
			exit(0)
			return
		}

		// Find free display
		display, err := x11.FindFreeDisplay()
		if err != nil {
			if *dryRun {
				fmt.Fprintf(os.Stderr, "Dry run warning: Failed to find free display (likely no X running or tools missing): %v. Assuming :0.\n", err)
				display = 0
			} else {
				fmt.Fprintf(os.Stderr, "Failed to find free display: %v\n", err)
				closeLoginSession(loginSession)
				exit(1)
				return
			}
		}

		vt, err := x11.GetVT(cfg.XTTY, display)
		if err != nil {
			if *dryRun {
				fmt.Fprintf(os.Stderr, "Dry run warning: Failed to get VT: %v. Assuming VT7.\n", err)
				vt = "7"
			} else {
				fmt.Fprintf(os.Stderr, "Failed to get VT: %v\n", err)
				closeLoginSession(loginSession)
				exit(1)
				return
			}
		}

		parts := strings.Fields(selectedSession.Exec)

		if *dryRun {
			fmt.Printf("Dry run: would launch X session: %v on display :%d VT%s\n", parts, display, vt)
			fmt.Printf("X server args: %v\n", cfg.ServerArgs)
			closeLoginSession(loginSession)
			return
		}

		if isLoginMode {
			env, sessionCwd, ttyCleanup, ok := prepareGettySession("X")
			if !ok {
				return
			}
			uid := acct.UID
			gid := acct.GID
			err = LaunchSupervisedXSessionFn(x11.LaunchOptions{
				Bin:        parts,
				Display:    display,
				VT:         vt,
				ConsoleKit: cfg.ConsoleKit,
				CKTimeout:  cfg.CKTimeout,
				AltStartX:  cfg.AltStartX,
				StartXLog:  cfg.StartXLog,
				ServerArgs: cfg.ServerArgs,
				Env:        env,
				Dir:        sessionCwd,
				UID:        &uid,
				GID:        &gid,
				Groups:     acct.Groups,
			})
			if ttyErr := ttyCleanup(); ttyErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to restore TTY ownership: %v\n", ttyErr)
			}
			closeLoginSession(loginSession)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to launch X session: %v\n", err)
				exit(1)
				return
			}
			exit(0)
			return
		}

		env := prepareUserSession("X")
		err = LaunchXSessionFn(parts, display, vt, cfg.ConsoleKit, cfg.CKTimeout, cfg.AltStartX, cfg.StartXLog, cfg.ServerArgs, env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to launch X session: %v\n", err)
			exit(1)
			return
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown session type: %s\n", selectedSession.Type)
		closeLoginSession(loginSession)
		exit(1)
		return
	}
}

func setPAMSessionMetadata(loginSession auth.LoginSession, stype, sname, vt string) {
	if loginSession == nil {
		return
	}
	var sessionType string
	switch stype {
	case "W":
		sessionType = "wayland"
	case "X":
		sessionType = "x11"
	case "C":
		sessionType = "tty"
	}
	setOpt := func(k, v string) {
		if v == "" {
			return
		}
		if err := loginSession.SetEnv(k, v); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set PAM variable %s: %v\n", k, err)
		}
	}
	setOpt("XDG_SESSION_TYPE", sessionType)
	setOpt("XDG_SESSION_CLASS", "user")
	if (stype == "W" || stype == "X") && sname != "" {
		setOpt("XDG_SESSION_DESKTOP", sname)
	}
	if vt != "" {
		setOpt("XDG_VTNR", vt)
		setOpt("XDG_SEAT", "seat0")
	}
}

func resolveSessionCwd(homeDir string) (string, error) {
	return resolveAndChdir(homeDir, func(dir string) error {
		if dir == "/" {
			return nil
		}
		info, err := osStat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("%q is not a directory", dir)
		}
		return nil
	})
}

func resolveSessions(cfg *config.Config, homeDir string) ([]session.Session, error) {
	if len(cfg.BinList) == 0 {
		sessions, err := DiscoverSessionsFn(homeDir)
		if err != nil {
			return nil, fmt.Errorf("Error discovering sessions: %w", err)
		}
		if len(sessions) == 0 {
			return nil, fmt.Errorf("No sessions found.")
		}
		return sessions, nil
	}

	sessions := make([]session.Session, 0, len(cfg.BinList))
	for i := 0; i < len(cfg.BinList); i++ {
		name := cfg.BinList[i]
		if i < len(cfg.NameList) {
			name = cfg.NameList[i]
		}
		sType := "X"
		if i < len(cfg.FlagList) {
			sType = cfg.FlagList[i]
		}
		sessions = append(sessions, session.Session{
			Name: name,
			Exec: cfg.BinList[i],
			Type: sType,
		})
	}
	return sessions, nil
}

func normalizeSessionType(raw string) string {
	flagVal := strings.ToUpper(raw)
	// gocdm script: [Cc] -> Console, [Xx] -> X, [Ww] -> Wayland.
	if strings.HasPrefix(flagVal, "C") {
		return "C"
	}
	if strings.HasPrefix(flagVal, "W") {
		return "W"
	}
	if strings.HasPrefix(flagVal, "X") {
		return "X"
	}
	return flagVal
}

func closeLoginSession(loginSession auth.LoginSession) {
	if loginSession == nil {
		return
	}
	if err := loginSession.CloseSession(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close PAM session: %v\n", err)
	}
}

func resolveAndChdir(homeDir string, chdir func(string) error) (string, error) {
	if err := config.ValidateHomeDir(homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid home directory %q (%v); falling back to /\n", homeDir, err)
		if rootErr := chdir("/"); rootErr != nil {
			return "", fmt.Errorf("fallback chdir /: %w", rootErr)
		}
		return "/", nil
	}
	if err := chdir(homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot change directory to %q (%v); falling back to /\n", homeDir, err)
		if rootErr := chdir("/"); rootErr != nil {
			return "", fmt.Errorf("fallback chdir /: %w", rootErr)
		}
		return "/", nil
	}
	return homeDir, nil
}

func formatDebugEnv(env []string) string {
	sorted := append([]string(nil), env...)
	sort.Strings(sorted)
	var b strings.Builder
	b.WriteString("--- DEBUG ENV ---\n")
	for _, e := range sorted {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if debugEnvAllowlist[parts[0]] {
			fmt.Fprintf(&b, "%s\n", e)
		} else {
			fmt.Fprintf(&b, "%s=[REDACTED]\n", parts[0])
		}
	}
	b.WriteString("-----------------\n")
	return b.String()
}

// isSecureTTYPath validates that the provided string is a valid virtual terminal path.
// It must reside securely in /dev/ (no subdirectories), start with "/dev/tty",
// be followed by a valid number, and actually be a character device on the filesystem.
func isSecureTTYPath(path string) bool {
	return isSecureTTYPathWithStat(path, osStat)
}

func currentUsername() string {
	if envUser := os.Getenv("USER"); envUser != "" {
		return envUser
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}

func validateTTY(dryRun bool) error {
	if dryRun {
		return nil
	}
	if !IsTerminal(int(os.Stdin.Fd())) || !IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("gocdm must be launched from an interactive TTY")
	}
	return nil
}
