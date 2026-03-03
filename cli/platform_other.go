//go:build !linux && !freebsd && !openbsd && !netbsd && !dragonfly

package cli

func platformSupportTier() string { return "unsupported" }
func supportsLoginMode() bool     { return false }
func supportsXSessions() bool     { return false }
