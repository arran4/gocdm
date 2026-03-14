//go:build windows

package cli

import (
	"errors"
	"fmt"
)

var (
	ErrUsernameRequired           = errors.New("username is required for privilege drop")
	ErrDropPrivilegesNotSupported = fmt.Errorf("privilege drop is not supported on windows: %w", errors.ErrUnsupported)
)

func DropPrivileges(username string) error {
	if username == "" {
		return ErrUsernameRequired
	}
	return ErrDropPrivilegesNotSupported
}
