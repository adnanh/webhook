// +build !windows

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func setupSignals() {
	log.Printf("setting up os signal watcher\n")

	signals = make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	signal.Notify(signals, syscall.SIGHUP)

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
		default:
			log.Printf("caught unhandled signal %+v\n", sig)
		}
	}
}
