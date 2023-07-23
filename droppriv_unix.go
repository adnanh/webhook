// +build linux !windows

package main

import (
	"syscall"
)

func dropPrivileges(uid, gid int) error {
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
