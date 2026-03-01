//go:build !windows

package cli

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"
)

func DropPrivileges(username string) error {
	if username == "" {
		return fmt.Errorf("username is required for privilege drop")
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("-login requires root privileges to change credentials")
	}

	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("lookup user %q: %w", username, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("invalid uid %q: %w", u.Uid, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("invalid gid %q: %w", u.Gid, err)
	}

	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("setgid %d: %w", gid, err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("setuid %d: %w", uid, err)
	}
	return nil
}
