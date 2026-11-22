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
	if kernelGeteuid() != 0 || acct == nil {
		return noopTTYCleanup, vt, nil
	}

	fd := int(os.Stdin.Fd())
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return noopTTYCleanup, vt, fmt.Errorf("fstat tty fd %d (%s): %w", fd, ttyPath, err)
	}
	if (uint32(st.Mode) & uint32(unix.S_IFMT)) != uint32(unix.S_IFCHR) {
		return noopTTYCleanup, vt, fmt.Errorf("stdin tty %s is not a character device", ttyPath)
	}
	origUID := int(st.Uid)
	origGID := int(st.Gid)
	origMode := uint32(st.Mode) & 0o7777

	ttyGID := acct.GID
	if grp, err := user.LookupGroup("tty"); err == nil {
		if parsedGID, convErr := strconv.Atoi(grp.Gid); convErr == nil && parsedGID >= 0 {
			ttyGID = parsedGID
		}
	}

	if err := unix.Fchown(fd, acct.UID, ttyGID); err != nil {
		return noopTTYCleanup, vt, fmt.Errorf("fchown tty %s: %w", ttyPath, err)
	}
	if err := unix.Fchmod(fd, 0o620); err != nil {
		_ = unix.Fchown(fd, origUID, origGID)
		return noopTTYCleanup, vt, fmt.Errorf("fchmod tty %s: %w", ttyPath, err)
	}

	var once sync.Once
	cleanup := func() error {
		var restoreErr error
		once.Do(func() {
			restoreErr = errors.Join(
				unix.Fchown(fd, origUID, origGID),
				unix.Fchmod(fd, origMode),
			)
		})
		return restoreErr
	}
	return cleanup, vt, nil
}
