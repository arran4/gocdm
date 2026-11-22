//go:build !windows

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"

	"github.com/arran4/gocdm/config"
	"golang.org/x/sys/unix"
)

// TTYCleanup restores original TTY ownership and permissions after a supervised session exits.
type TTYCleanup func() error

func noopTTYCleanup() error { return nil }

func detectCurrentVT() string {
	for _, fd := range []string{"0", "1"} {
		if ttyPath, err := osReadlink("/proc/self/fd/" + fd); err == nil && isSecureTTYPath(ttyPath) {
			return strings.TrimPrefix(ttyPath, "/dev/tty")
		}
	}
	return ""
}

func prepareTTYForAccount(acct *config.AccountInfo) (TTYCleanup, string, error) {
	ttyPath, err := osReadlink("/proc/self/fd/0")
	if err != nil || !isSecureTTYPath(ttyPath) {
		return noopTTYCleanup, detectCurrentVT(), nil
	}

	vt := strings.TrimPrefix(ttyPath, "/dev/tty")
	if osGeteuid() != 0 || acct == nil {
		return noopTTYCleanup, vt, nil
	}

	var st unix.Stat_t
	if err := unix.Stat(ttyPath, &st); err != nil {
		return noopTTYCleanup, vt, fmt.Errorf("stat tty %s: %w", ttyPath, err)
	}
	origUID := int(st.Uid)
	origGID := int(st.Gid)
	origMode := os.FileMode(st.Mode) & os.ModePerm

	ttyGID := acct.GID
	if grp, err := user.LookupGroup("tty"); err == nil {
		if parsedGID, convErr := strconv.Atoi(grp.Gid); convErr == nil && parsedGID >= 0 {
			ttyGID = parsedGID
		}
	}

	if err := os.Chown(ttyPath, acct.UID, ttyGID); err != nil {
		return noopTTYCleanup, vt, fmt.Errorf("chown tty %s: %w", ttyPath, err)
	}
	if err := os.Chmod(ttyPath, 0o620); err != nil {
		_ = os.Chown(ttyPath, origUID, origGID)
		return noopTTYCleanup, vt, fmt.Errorf("chmod tty %s: %w", ttyPath, err)
	}

	var once sync.Once
	cleanup := func() error {
		var restoreErr error
		once.Do(func() {
			restoreErr = errors.Join(
				os.Chown(ttyPath, origUID, origGID),
				os.Chmod(ttyPath, origMode),
			)
		})
		return restoreErr
	}
	return cleanup, vt, nil
}
