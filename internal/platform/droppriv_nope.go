//go:build linux || windows
// +build linux windows

package platform

import (
	"errors"
	"runtime"
)

func DropPrivileges(uid, gid int) error {
	return errors.New("setuid and setgid not supported on " + runtime.GOOS)
}
