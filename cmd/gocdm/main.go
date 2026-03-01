package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/arran4/gocdm/auth"
	"github.com/arran4/gocdm/config"
	"github.com/arran4/gocdm/dialog"
	"github.com/arran4/gocdm/session"
	"github.com/arran4/gocdm/x11"
	"golang.org/x/term"
)

var version = "dev"
var isTerminal = term.IsTerminal

// TODO figure out how to support cygwin/wsl for parsing /etc/passwd and pam_env.conf properly
var passwdFilePath = "/etc/passwd"
var pamEnvConfPath = "/etc/security/pam_env.conf"

func main() {
	run(os.Args[1:], os.Exit)
}

func run(args []string, exit func(int)) {
	fs := flag.NewFlagSet("gocdm", flag.ContinueOnError)
	configPathFlag := fs.String("config", "", "Path to config file")
	dryRun := fs.Bool("dry-run", false, "Dry run mode (print command instead of executing)")
	forceMenu := fs.Bool("menu", false, "Force menu display even if only one session is found")
	showVersion := fs.Bool("version", false, "Show version information")
	loginMode := fs.Bool("login", false, "Prompt for username/password and authenticate with PAM")
	pamService := fs.String("pam-service", "login", "PAM service name used with -login")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			exit(0)
		} else {
			exit(2)
		}
		return
	}

	if *showVersion {
		fmt.Printf("gocdm version %s\n", version)
		exit(0)
		return
	}

	var configPath string
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
		sessions, err = session.DiscoverSessions(home)
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

		idx, err := dialog.ShowMenu("Console Display Manager", optionNames, details, cfg.CountFrom, defaultIdx, cfg.DialogRC)
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
	if err := validateTTY(*dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		exit(1)
		return
	}

	username := currentUsername()
	if *loginMode {
		authenticator := auth.NewPAMAuthenticator(*pamService)
		promptedUser, password, err := auth.PromptCredentials(os.Stdin, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Authentication prompt failed: %v\n", err)
			exit(1)
			return
		}
		if err := authenticator.Authenticate(promptedUser, password); err != nil {
			fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
			exit(1)
			return
		}
		username = promptedUser
	}

	sessionEnv, err := config.BuildSessionEnv(os.Environ(), username, passwdFilePath, pamEnvConfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to build secure session environment: %v\n", err)
		sessionEnv = os.Environ()
	}

	// Save state
	if err := config.SaveState(selectedSession.Name); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to save state: %v\n", err)
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
		if *loginMode {
			if err := dropPrivileges(username); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to switch user context: %v\n", err)
				exit(1)
				return
			}
		}

		binary, err := exec.LookPath(bin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Command not found: %s\n", bin)
			exit(1)
			return
		}

		env := append([]string{}, sessionEnv...)
		env = append(env, fmt.Sprintf("GOCDM_SPAWN=%d", os.Getpid()))
		env = append(env, "XDG_SESSION_TYPE=wayland")

		if err := execProgram(binary, append([]string{bin}, args...), env); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to exec: %v\n", err)
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
		if *loginMode {
			if err := dropPrivileges(username); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to switch user context: %v\n", err)
				exit(1)
				return
			}
		}

		binary, err := exec.LookPath(bin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Command not found: %s\n", bin)
			exit(1)
			return
		}

		env := append([]string{}, sessionEnv...)
		env = append(env, fmt.Sprintf("GOCDM_SPAWN=%d", os.Getpid()))

		if err := execProgram(binary, append([]string{bin}, args...), env); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to exec: %v\n", err)
			exit(1)
			return
		}

	case "X":
		// X program

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
		if *loginMode {
			if err := dropPrivileges(username); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to switch user context: %v\n", err)
				exit(1)
				return
			}
		}

		err = x11.LaunchXSession(parts, display, vt, cfg.ConsoleKit, cfg.CKTimeout, cfg.AltStartX, cfg.StartXLog, cfg.ServerArgs, sessionEnv)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to launch X session: %v\n", err)
			exit(1)
			return
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown session type: %s\n", selectedSession.Type)
		exit(1)
		return
	}
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
	if !isTerminal(int(os.Stdin.Fd())) || !isTerminal(int(os.Stdout.Fd())) || !isTerminal(int(os.Stderr.Fd())) {
		return fmt.Errorf("gocdm must be launched from an interactive TTY")
	}
	return nil
}
