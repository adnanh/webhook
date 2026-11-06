//go:build windows
// +build windows

package app

import (
	"errors"
	"runtime"
)

func dropPrivileges(uid, gid int) error {
	return errors.New("setuid and setgid not supported on " + runtime.GOOS)
}
