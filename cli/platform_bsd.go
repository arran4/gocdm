//go:build freebsd || openbsd || netbsd || dragonfly

package cli

func platformSupportTier() string { return "secondary" }
func supportsLoginMode() bool     { return true }
func supportsXSessions() bool     { return true }
