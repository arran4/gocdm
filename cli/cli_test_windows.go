//go:build windows

package cli

import "testing"

func mockUnsupported(t *testing.T) {
	origLoginMode := SupportsLoginModeFn
	origXSessions := SupportsXSessionsFn
	origTier := PlatformSupportTierFn

	SupportsLoginModeFn = func() bool { return true }
	SupportsXSessionsFn = func() bool { return true }
	PlatformSupportTierFn = func() string { return "primary" }

	t.Cleanup(func() {
		SupportsLoginModeFn = origLoginMode
		SupportsXSessionsFn = origXSessions
		PlatformSupportTierFn = origTier
	})
}
