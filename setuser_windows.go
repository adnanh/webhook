//go:build windows
// +build windows

package main

import "os/exec"

func setUser(cmd *exec.Cmd, username string) {
	// NOOP: Windows doesn't have setuid setgid equivalent to the Unix world.
}
