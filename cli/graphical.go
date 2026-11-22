package cli

import (
	"fmt"
	"strings"

	"github.com/arran4/gocdm/config"
	"github.com/arran4/gocdm/session"
)

// ParseGraphicalExec splits a graphical session command line into argv tokens without invoking a shell.
func ParseGraphicalExec(execCmd string) ([]string, error) {
	trimmed := strings.TrimSpace(execCmd)
	if trimmed == "" {
		return nil, fmt.Errorf("empty graphical command")
	}
	parts, err := config.SplitCommand(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse graphical command: %w", err)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty graphical command")
	}
	return parts, nil
}

// ResolveDesktopMetadata computes XDG_SESSION_DESKTOP/DESKTOP_SESSION and XDG_CURRENT_DESKTOP
// from a discovered session while falling back gracefully for manual binlist entries.
func ResolveDesktopMetadata(sess session.Session) (desktopSession, currentDesktop string) {
	desktopSession = sess.DesktopID
	if desktopSession == "" {
		desktopSession = sess.Name
	}
	if len(sess.DesktopNames) > 0 {
		currentDesktop = strings.Join(sess.DesktopNames, ":")
	} else {
		currentDesktop = desktopSession
	}
	return desktopSession, currentDesktop
}

// ResolveUseConsoleKit decides whether ck-launch-session should wrap X11 sessions.
// Explicit cdmrc settings are honored strictly; otherwise legacy ConsoleKit wrapping is skipped
// when PAM/logind has already established XDG_SESSION_ID or when ck-launch-session is absent.
func ResolveUseConsoleKit(cfg *config.Config, env []string) bool {
	if cfg == nil {
		return false
	}
	if cfg.ConsoleKitExplicit {
		return cfg.ConsoleKit
	}
	if !cfg.ConsoleKit {
		return false
	}
	if config.EnvSliceToMap(env)["XDG_SESSION_ID"] != "" {
		return false
	}
	if _, err := ExecLookPath("ck-launch-session"); err != nil {
		return false
	}
	return true
}
