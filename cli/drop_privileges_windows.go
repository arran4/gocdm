//go:build windows

package cli

import "fmt"

func DropPrivileges(username string) error {
	if username == "" {
		return fmt.Errorf("username is required for privilege drop")
	}
	return fmt.Errorf("-login is not supported on windows")
}
