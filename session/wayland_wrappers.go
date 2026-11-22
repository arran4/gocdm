//go:build !nowaylandwrappers

package session

import "strings"

var PlasmaWaylandWrapperPath = "/usr/libexec/plasma-dbus-run-session-if-needed"

var plasmaWaylandWrapperCandidates = []string{
	"/usr/lib64/libexec/plasma-dbus-run-session-if-needed",
	"/usr/lib/plasma-dbus-run-session-if-needed",
	"plasma-dbus-run-session-if-needed",
}

func init() {
	RegisterPostParseDesktopHook(ApplyWaylandWrappers)
}

// ApplyWaylandWrappers modifies the exec command to include necessary wrappers
// (like the SDDM dbus/logind wrapper for KDE Plasma) without double-wrapping
// Plasma 6 desktop entries that already include it or dbus-run-session.
func ApplyWaylandWrappers(execCmd string, d *Discoverer) string {
	if !strings.Contains(execCmd, "startplasma-wayland") {
		return execCmd
	}
	if strings.Contains(execCmd, "plasma-dbus-run-session-if-needed") || strings.Contains(execCmd, "dbus-run-session") {
		return execCmd
	}

	candidates := make([]string, 0, 1+len(plasmaWaylandWrapperCandidates))
	if PlasmaWaylandWrapperPath != "" {
		candidates = append(candidates, PlasmaWaylandWrapperPath)
	}
	for _, c := range plasmaWaylandWrapperCandidates {
		if c != PlasmaWaylandWrapperPath {
			candidates = append(candidates, c)
		}
	}

	for _, candidate := range candidates {
		if resolved, err := d.ExecLookPath(candidate); err == nil {
			if resolved != "" {
				return resolved + " " + execCmd
			}
			return candidate + " " + execCmd
		}
	}
	return execCmd
}
