//go:build windows

package cli

import (
	"errors"
	"fmt"
)

func DropPrivileges(username string) error {
	if username == "" {
		return errors.New("username is required for privilege drop")
	}
	return fmt.Errorf("privilege drop is not supported on windows: %w", errors.ErrUnsupported)
}
