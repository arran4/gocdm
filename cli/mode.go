package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

const unsetLoginUID uint32 = 0xffffffff

// OperatingMode represents the resolved execution mode for GoCDM.
type OperatingMode string

const (
	// ModeGettyLogin runs GoCDM as a privileged direct getty replacement that authenticates the user.
	ModeGettyLogin OperatingMode = "login"
	// ModeUserSession runs GoCDM inside an already-authenticated user session (e.g. profile autostart).
	ModeUserSession OperatingMode = "user"
)

// ModeSelection represents the requested operating mode before runtime resolution.
type ModeSelection string

const (
	ModeAuto        ModeSelection = "auto"
	ModeSelectLogin ModeSelection = "login"
	ModeSelectUser  ModeSelection = "user"
)

var (
	osGetuid     = os.Getuid
	osGeteuid    = os.Geteuid
	readLoginUID = readSystemLoginUID
)

// ProcessContext captures the runtime identity and environment signals used to resolve OperatingMode.
type ProcessContext struct {
	UID           int
	EUID          int
	GOOS          string
	SupportTier   string
	SupportsLogin bool
	Getenv        func(string) string
	Readlink      func(string) (string, error)
	Stat          func(string) (os.FileInfo, error)
	IsTerminal    func(int) bool
	ReadLoginUID  func() (uint32, bool)
}

func defaultProcessContext() ProcessContext {
	return ProcessContext{
		UID:           osGetuid(),
		EUID:          osGeteuid(),
		GOOS:          runtime.GOOS,
		SupportTier:   PlatformSupportTierFn(),
		SupportsLogin: SupportsLoginModeFn(),
		Getenv:        os.Getenv,
		Readlink:      osReadlink,
		Stat:          osStat,
		IsTerminal:    IsTerminal,
		ReadLoginUID:  readLoginUID,
	}
}

// ParseModeFlag parses the -mode CLI option into a ModeSelection.
func ParseModeFlag(raw string) (ModeSelection, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return ModeAuto, nil
	case "login", "getty":
		return ModeSelectLogin, nil
	case "user", "autostart", "session":
		return ModeSelectUser, nil
	default:
		return "", fmt.Errorf("invalid -mode %q: expected auto, login (or getty), or user (or autostart)", raw)
	}
}

// ResolveModeSelection reconciles explicit -mode and -login flags into a single ModeSelection.
func ResolveModeSelection(modeFlagSet bool, modeRaw string, loginFlagSet bool, loginFlagVal bool) (ModeSelection, error) {
	parsedMode := ModeAuto
	if modeFlagSet {
		var err error
		parsedMode, err = ParseModeFlag(modeRaw)
		if err != nil {
			return "", err
		}
	}
	if loginFlagSet {
		loginSel := ModeSelectUser
		if loginFlagVal {
			loginSel = ModeSelectLogin
		}
		if modeFlagSet && parsedMode != ModeAuto && parsedMode != loginSel {
			return "", fmt.Errorf("conflicting mode flags: -login=%v cannot be combined with -mode=%s", loginFlagVal, modeRaw)
		}
		return loginSel, nil
	}
	return parsedMode, nil
}

// ResolveOperatingMode determines whether GoCDM should operate as a direct getty login manager
// or an already-authenticated user launcher, failing clearly on invalid or inconsistent contexts.
func ResolveOperatingMode(sel ModeSelection, ctx ProcessContext, dryRun bool) (OperatingMode, string, error) {
	getenv := ctx.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}

	switch sel {
	case ModeSelectUser:
		return ModeUserSession, "explicit user session mode", nil

	case ModeSelectLogin:
		if !ctx.SupportsLogin {
			return "", "", fmt.Errorf("-login is not supported on %s (support tier: %s)", ctx.GOOS, ctx.SupportTier)
		}
		if !dryRun {
			if ctx.EUID != 0 {
				return "", "", fmt.Errorf("direct getty login mode (-login) requires root privileges (current uid=%d euid=%d); omit -login or pass -mode=user for an already-authenticated session", ctx.UID, ctx.EUID)
			}
			if getenv("DISPLAY") != "" || getenv("WAYLAND_DISPLAY") != "" {
				return "", "", fmt.Errorf("direct getty login mode (-login) cannot run inside an active graphical session (DISPLAY/WAYLAND_DISPLAY is set); use -mode=user instead")
			}
			if getenv("SSH_TTY") != "" || getenv("SSH_CONNECTION") != "" || getenv("SSH_CLIENT") != "" {
				return "", "", fmt.Errorf("direct getty login mode (-login) cannot run inside an SSH session; use -mode=user instead")
			}
		}
		return ModeGettyLogin, "explicit getty login mode", nil

	case ModeAuto:
		if !ctx.SupportsLogin || ctx.UID != 0 || ctx.EUID != 0 {
			return ModeUserSession, "auto-detected user session mode", nil
		}
		sessionEnvMarkers := []string{
			"DISPLAY",
			"WAYLAND_DISPLAY",
			"SSH_TTY",
			"SSH_CONNECTION",
			"SSH_CLIENT",
			"XDG_SESSION_ID",
			"GOCDM_SPAWN",
			"SHLVL",
			"USER",
			"LOGNAME",
			"SUDO_USER",
			"DOAS_USER",
		}
		for _, k := range sessionEnvMarkers {
			if getenv(k) != "" {
				return ModeUserSession, fmt.Sprintf("auto-detected user session mode (%s is set)", k), nil
			}
		}
		if ctx.ReadLoginUID != nil {
			if loginUID, ok := ctx.ReadLoginUID(); ok && loginUID != unsetLoginUID {
				return ModeUserSession, "auto-detected already-authenticated user session (audit loginuid already set)", nil
			}
		}
		if ctx.Readlink != nil {
			tty0, err0 := ctx.Readlink("/proc/self/fd/0")
			tty1, err1 := ctx.Readlink("/proc/self/fd/1")
			if err0 == nil && err1 == nil && tty0 == tty1 && isSecureTTYPathWithStat(tty0, ctx.Stat) {
				return ModeGettyLogin, fmt.Sprintf("auto-detected direct getty replacement on %s", tty0), nil
			}
		}
		return ModeUserSession, "auto-detected user session mode", nil

	default:
		return "", "", fmt.Errorf("invalid mode selection %q", sel)
	}
}

func isSecureTTYPathWithStat(path string, statFn func(string) (os.FileInfo, error)) bool {
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

	if statFn == nil {
		return false
	}
	info, err := statFn(path)
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
