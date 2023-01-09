//go:build !windows
// +build !windows

package platform

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/adnanh/webhook/internal/pidfile"
)

func SetupSignals(signals chan os.Signal, reloadFn func(), pidFile *pidfile.PIDFile) {
	log.Printf("setting up os signal watcher\n")

	signals = make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	signal.Notify(signals, syscall.SIGHUP)
	signal.Notify(signals, syscall.SIGTERM)
	signal.Notify(signals, os.Interrupt)

	go watchForSignals(signals, reloadFn, pidFile)
}

func watchForSignals(signals chan os.Signal, reloadFn func(), pidFile *pidfile.PIDFile) {
	log.Println("os signal watcher ready")

	for {
		sig := <-signals
		switch sig {
		case syscall.SIGUSR1:
			log.Println("caught USR1 signal")
			reloadFn()

		case syscall.SIGHUP:
			log.Println("caught HUP signal")
			reloadFn()

		case os.Interrupt, syscall.SIGTERM:
			log.Printf("caught %s signal; exiting\n", sig)
			if pidFile != nil {
				err := pidFile.Remove()
				if err != nil {
					log.Print(err)
				}
			}
			os.Exit(0)

		default:
			log.Printf("caught unhandled signal %+v\n", sig)
		}
	}
}
