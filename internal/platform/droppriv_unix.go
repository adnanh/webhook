//go:build !windows && !linux
// +build !windows,!linux

package platform

import (
	"syscall"
)

func DropPrivileges(uid, gid int) error {
	err := syscall.Setgid(gid)
	if err != nil {
		return err
	}

	err = syscall.Setuid(uid)
	if err != nil {
		return err
	}

	return nil
}
