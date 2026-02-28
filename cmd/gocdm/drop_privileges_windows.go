//go:build windows

package main

import "fmt"

func dropPrivileges(username string) error {
	if username == "" {
		return fmt.Errorf("username is required for privilege drop")
	}
	return fmt.Errorf("-login is not supported on windows")
}
