//go:build !windows && !darwin

package cli

import "testing"

func mockUnsupported(t *testing.T) {
	// No-op for supported platforms
}
