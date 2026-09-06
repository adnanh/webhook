//go:build !windows
// +build !windows

package main

import (
	"log"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

// sets user for the command to execute
func setUser(cmd *exec.Cmd, username string) {
	user, err := user.Lookup(username)
	if err != nil {
		log.Printf("[%s] error lookup user: %s\n", username, err)
		return
	}
	uid, err := strconv.ParseUint(user.Uid, 10, 32)
	if err != nil {
		log.Printf("Uid [%s] is not an decimal value: %s\n", user.Uid, err)
		return
	}
	gid, err := strconv.ParseUint(user.Gid, 10, 32)
	if err != nil {
		log.Printf("Uid [%s] is not an decimal value: %s\n", user.Uid, err)
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}}
}
