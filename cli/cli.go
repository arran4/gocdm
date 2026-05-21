package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
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
var DiscoverSessionsFn = session.DiscoverSessions
var osStat = os.Stat
var osReadlink = os.Readlink

var SupportsLoginModeFn = supportsLoginMode
var SupportsXSessionsFn = supportsXSessions
var PlatformSupportTierFn = platformSupportTier

var PasswdFilePath = "/etc/passwd"
var PamEnvConfPath = "/etc/security/pam_env.conf"

func Run(args []string, exit func(int)) {
	fs := flag.NewFlagSet("gocdm", flag.ContinueOnError)
	configPathFlag := fs.String("config", "", "Path to config file")
	dryRun := fs.Bool("dry-run", false, "Dry run mode (print command instead of executing)")
	forceMenu := fs.Bool("menu", false, "Force menu display even if only one session is found")
	showVersion := fs.Bool("version", false, "Show version information")
	loginMode := fs.Bool("login", false, "Prompt for username/password and authenticate with PAM")
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

	// Getty auto-detection for login mode consolidation
	if !*loginMode && os.Getenv("DISPLAY") == "" {
		for _, fd := range []string{"0", "1", "2"} {
			if ttyPath, err := osReadlink("/proc/self/fd/" + fd); err == nil && isSecureTTYPath(ttyPath) {
				*loginMode = true
				break
			}
		}
	}

	var configPath string
	if *configPathFlag != "" && fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "cannot use both positional config path and -config; use one source")
		exit(2)
		return
	}
	if *configPathFlag != "" {
		configPath = *configPathFlag
	} else if fs.NArg() > 0 {
		// Backward compatibility: first positional argument is config path
		configPath = fs.Arg(0)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		exit(1)
		return
	}

	// Load state (last session)
	state, err := config.LoadState()
	if err != nil {
		// Ignore error, just start fresh
		state = &config.State{}
	}

	// Populate sessions if binlist is empty
	var sessions []session.Session
	if len(cfg.BinList) == 0 {
		home, _ := os.UserHomeDir()
		sessions, err = DiscoverSessionsFn(home)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error discovering sessions: %v\n", err)
			exit(1)
			return
		}
		if len(sessions) == 0 {
			fmt.Fprintln(os.Stderr, "No sessions found.")
			exit(1)
			return
		}
	} else {
		// Convert config lists to sessions
		// Assumes lists are equal length. cfg.BinList, cfg.NameList, cfg.FlagList
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
	}

	if err := validateTTY(*dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		exit(1)
		return
	}

	username := currentUsername()
	var loginSession auth.LoginSession
	if *loginMode && !SupportsLoginModeFn() {
		fmt.Fprintf(os.Stderr, "-login is not supported on %s (support tier: %s)\n", runtime.GOOS, PlatformSupportTierFn())
		exit(1)
		return
	}
	if *loginMode {
		authenticator := NewAuthenticator(*pamService)
		var promptedUser, password string
		var err error

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
		username = promptedUser
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
			exit(2)
			return
		}
		selectedIdx = idx
	}

	if selectedIdx < 0 || selectedIdx >= len(sessions) {
		fmt.Fprintf(os.Stderr, "Invalid selection index: %d\n", selectedIdx)
		exit(1)
		return
	}

	selectedSession := sessions[selectedIdx]

	sessionEnv, err := config.BuildSessionEnv(os.Environ(), username, PasswdFilePath, PamEnvConfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to build secure session environment: %v\n", err)
		sessionEnv = os.Environ()
	}

	var homeDir string
	if *loginMode {
		entry, err := config.LoadPasswdEntry(PasswdFilePath, username)
		if err == nil {
			homeDir = entry.HomeDir
		}
	}
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}

	setupUserContext := func(baseEnv []string, stype, sname string) []string {
		envMap := config.EnvSliceToMap(baseEnv)

		if *loginMode && loginSession != nil {
			if err := loginSession.OpenSession(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: PAM open session failed: %v\n", err)
				exit(1)
				return nil
			}

			// We already loaded the passwd entry into homeDir, but let's re-use the parsed fields
			// Wait, we need the whole entry. We can just load it again, it's fast, or we can use the environment map directly.
			// Actually, let's load it.
			entry, err := config.LoadPasswdEntry(PasswdFilePath, username)
			if err == nil {
				envMap["USER"] = entry.Username
				envMap["LOGNAME"] = entry.Username
				envMap["HOME"] = entry.HomeDir
				envMap["SHELL"] = entry.Shell
			}

			pamEnv := loginSession.Env()
			if len(pamEnv) > 0 {
				pamEnvMap := config.EnvSliceToMap(pamEnv)
				for k, v := range pamEnvMap {
					envMap[k] = v
				}
			}
		}

		checkAndSet := func(key, val string) {
			if existing, ok := envMap[key]; ok && existing != "" && existing != val {
				fmt.Fprintf(os.Stderr, "Warning: overriding existing environment variable %s=%s with %s\n", key, existing, val)
			}
			envMap[key] = val
		}

		if stype == "W" {
			checkAndSet("XDG_SESSION_TYPE", "wayland")
		} else if stype == "X" {
			checkAndSet("XDG_SESSION_TYPE", "x11")
		} else if stype == "C" {
			checkAndSet("XDG_SESSION_TYPE", "tty")
		}
		checkAndSet("XDG_SESSION_CLASS", "user")
		checkAndSet("XDG_SESSION_DESKTOP", sname)
		checkAndSet("DESKTOP_SESSION", sname)
		checkAndSet("XDG_CURRENT_DESKTOP", sname)

		env := config.EnvMapToSlice(envMap)

		if err := config.SaveStateAt(homeDir, selectedSession.Name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to save state: %v\n", err)
		}

		if *debugEnv {
			fmt.Println("--- DEBUG ENV ---")
			for _, e := range env {
				if strings.HasPrefix(e, "DBUS_SESSION_BUS_ADDRESS=") ||
				   strings.HasPrefix(e, "HOME=") ||
				   strings.HasPrefix(e, "USER=") ||
				   strings.HasPrefix(e, "LOGNAME=") ||
				   strings.HasPrefix(e, "SHELL=") ||
				   strings.HasPrefix(e, "XDG_RUNTIME_DIR=") ||
				   strings.HasPrefix(e, "XDG_SESSION_TYPE=") ||
				   strings.HasPrefix(e, "DESKTOP_SESSION=") {
					fmt.Println(e)
				} else if strings.Contains(e, "=") {
					parts := strings.SplitN(e, "=", 2)
					fmt.Printf("%s=[REDACTED]\n", parts[0])
				}
			}
			fmt.Println("-----------------")
		}
		return env
	}

	// Normalize flag
	flagVal := strings.ToUpper(selectedSession.Type)
	// gocdm script: [Cc] -> Console, [Xx] -> X, [Ww] -> Wayland.
	if strings.HasPrefix(flagVal, "C") {
		flagVal = "C"
	} else if strings.HasPrefix(flagVal, "W") {
		flagVal = "W"
	} else if strings.HasPrefix(flagVal, "X") {
		flagVal = "X"
	}

	switch flagVal {
	case "W":
		// Wayland session
		parts := strings.Fields(selectedSession.Exec)
		if len(parts) == 0 {
			fmt.Fprintln(os.Stderr, "Empty command")
			exit(1)
			return
		}
		bin := parts[0]
		args := parts[1:]

		if *dryRun {
			fmt.Printf("Dry run: would execute Wayland program: %s %v\n", bin, args)
			return
		}
		binary, err := ExecLookPath(bin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Command not found: %s\n", bin)
			if loginSession != nil {
				if closeErr := loginSession.CloseSession(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "Failed to close PAM session: %v\n", closeErr)
				}
			}
			exit(1)
			return
		}

		env := setupUserContext(sessionEnv, "W", selectedSession.Name)
		if env == nil { return }
		env = append(env, fmt.Sprintf("GOCDM_SPAWN=%d", os.Getpid()))

		if *loginMode {
			if err := DropPrivilegesFn(username); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to switch user context: %v\n", err)
				if loginSession != nil {
					_ = loginSession.CloseSession()
				}
				exit(1)
				return
			}
		}

		if err := ExecProgramFn(binary, append([]string{bin}, args...), env); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to exec: %v\n", err)
			// At this point privileges are dropped, so CloseSession likely fails, but we try anyway
			if loginSession != nil {
				if closeErr := loginSession.CloseSession(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "Failed to close PAM session: %v\n", closeErr)
				}
			}
			exit(1)
			return
		}

	case "C":
		// Console program
		parts := strings.Fields(selectedSession.Exec)
		if len(parts) == 0 {
			fmt.Fprintln(os.Stderr, "Empty command")
			exit(1)
			return
		}
		bin := parts[0]
		args := parts[1:]

		if *dryRun {
			fmt.Printf("Dry run: would execute console program: %s %v\n", bin, args)
			return
		}
		binary, err := ExecLookPath(bin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Command not found: %s\n", bin)
			if loginSession != nil {
				if closeErr := loginSession.CloseSession(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "Failed to close PAM session: %v\n", closeErr)
				}
			}
			exit(1)
			return
		}

		env := setupUserContext(sessionEnv, "C", selectedSession.Name)
		if env == nil { return }
		env = append(env, fmt.Sprintf("GOCDM_SPAWN=%d", os.Getpid()))

		if *loginMode {
			if err := DropPrivilegesFn(username); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to switch user context: %v\n", err)
				if loginSession != nil {
					_ = loginSession.CloseSession()
				}
				exit(1)
				return
			}
		}

		if err := ExecProgramFn(binary, append([]string{bin}, args...), env); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to exec: %v\n", err)
			if loginSession != nil {
				if closeErr := loginSession.CloseSession(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "Failed to close PAM session: %v\n", closeErr)
				}
			}
			exit(1)
			return
		}

	case "X":
		// X program
		if !SupportsXSessionsFn() {
			fmt.Fprintf(os.Stderr, "X session launch is not supported on %s (support tier: %s)\n", runtime.GOOS, PlatformSupportTierFn())
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
					exit(1)
					return
				}
			}

			if *dryRun {
				fmt.Printf("Dry run: would switch to existing X session on display :%d VT%s\n", cfg.Display, vt)
				return
			}

			if err := x11.SwitchVT(vt); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to switch VT: %v\n", err)
				exit(1)
				return
			}

			// Successfully switched, exit normal
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
				exit(1)
				return
			}
		}

		parts := strings.Fields(selectedSession.Exec)

		if *dryRun {
			fmt.Printf("Dry run: would launch X session: %v on display :%d VT%s\n", parts, display, vt)
			fmt.Printf("X server args: %v\n", cfg.ServerArgs)
			return
		}
		env := setupUserContext(sessionEnv, "X", selectedSession.Name)
		if env == nil { return }

		if *loginMode {
			if err := DropPrivilegesFn(username); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to switch user context: %v\n", err)
				if loginSession != nil {
					_ = loginSession.CloseSession()
				}
				exit(1)
				return
			}
		}

		err = LaunchXSessionFn(parts, display, vt, cfg.ConsoleKit, cfg.CKTimeout, cfg.AltStartX, cfg.StartXLog, cfg.ServerArgs, env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to launch X session: %v\n", err)
			if loginSession != nil {
				if closeErr := loginSession.CloseSession(); closeErr != nil {
					fmt.Fprintf(os.Stderr, "Failed to close PAM session: %v\n", closeErr)
				}
			}
			exit(1)
			return
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown session type: %s\n", selectedSession.Type)
		exit(1)
		return
	}
}

// isSecureTTYPath validates that the provided string is a valid virtual terminal path.
// It must reside securely in /dev/ (no subdirectories), start with "/dev/tty",
// be followed by a valid number, and actually be a character device on the filesystem.
func isSecureTTYPath(path string) bool {
	if !strings.HasPrefix(path, "/dev/tty") || strings.Contains(path[8:], "/") {
		return false
	}

	suffix := path[8:]
	if len(suffix) == 0 {
		return false
	}
	for _, char := range suffix {
		if char < '0' || char > '9' {
			return false
		}
	}

	info, err := osStat(path)
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
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
