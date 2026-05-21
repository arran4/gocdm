//go:build !nowaylandwrappers

package session

import "strings"

var PlasmaWaylandWrapperPath = "/usr/libexec/plasma-dbus-run-session-if-needed"

// ApplyWaylandWrappers modifies the exec command to include necessary wrappers
// (like the SDDM dbus/logind wrapper for KDE Plasma).
func ApplyWaylandWrappers(execCmd string, d *Discoverer) string {
	if strings.Contains(execCmd, "startplasma-wayland") {
		wrapper := PlasmaWaylandWrapperPath
		if _, err := d.ExecLookPath(wrapper); err == nil {
			return wrapper + " " + execCmd
		}
	}
	return execCmd
}
