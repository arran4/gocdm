//go:build windows

package main

import "fmt"

func setGroupID(gid int) error {
	return fmt.Errorf("setting gid is not supported on windows")
}

func setUserID(uid int) error {
	return fmt.Errorf("setting uid is not supported on windows")
}
