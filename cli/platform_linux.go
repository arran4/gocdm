//go:build linux

package cli

func platformSupportTier() string { return "primary" }
func supportsLoginMode() bool     { return true }
func supportsXSessions() bool     { return true }
