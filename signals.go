//go:build !windows
// +build !windows

package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func setupSignals() {
	log.Printf("setting up os signal watcher\n")

	signals = make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	signal.Notify(signals, syscall.SIGHUP)
	signal.Notify(signals, syscall.SIGTERM)
	signal.Notify(signals, os.Interrupt)

	go watchForSignals()
}

func watchForSignals() {
	log.Println("os signal watcher ready")

	for {
		sig := <-signals
		switch sig {
		case syscall.SIGUSR1:
			log.Println("caught USR1 signal")
			reloadAllHooks()

		case syscall.SIGHUP:
			log.Println("caught HUP signal")
			reloadAllHooks()

		case os.Interrupt, syscall.SIGTERM:
			log.Printf("caught %s signal; exiting\n", sig)
			if pidFile != nil {
				err := pidFile.Remove()
				if err != nil {
					log.Print(err)
				}
			}
			if socket != "" && !strings.HasPrefix(socket, "@") {
				// we've been listening on a named Unix socket, delete it
				// before we exit so subsequent runs can re-bind the same
				// socket path
				err := os.Remove(socket)
				if err != nil {
					log.Printf("Failed to remove socket file %s: %v", socket, err)
				}
			}
			os.Exit(0)

		default:
			log.Printf("caught unhandled signal %+v\n", sig)
		}
	}
}
