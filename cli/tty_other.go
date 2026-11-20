//go:build windows

package cli

import "github.com/arran4/gocdm/config"

// TTYCleanup restores original TTY ownership and permissions after a supervised session exits.
type TTYCleanup func() error

func noopTTYCleanup() error { return nil }

func detectCurrentVT() string {
	return ""
}

func prepareTTYForAccount(acct *config.AccountInfo) (TTYCleanup, string, error) {
	return noopTTYCleanup, "", nil
}
