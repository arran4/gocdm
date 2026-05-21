//go:build nowaylandwrappers

package session

// ApplyWaylandWrappers is a stub for when wrappers are disabled via build tags.
func ApplyWaylandWrappers(execCmd string, d *Discoverer) string {
	return execCmd
}
