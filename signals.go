// +build !windows

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/adnanh/webhook/internal/service"
)

func setupSignals(svc *service.Service) {
	log.Printf("setting up os signal watcher\n")

	signals = make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	signal.Notify(signals, syscall.SIGHUP)
	signal.Notify(signals, syscall.SIGTERM)
	signal.Notify(signals, os.Interrupt)

	go watchForSignals(svc)
}

func watchForSignals(svc *service.Service) {
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

			if svc.TLSEnabled() {
				log.Println("attempting to reload TLS key pair")
				err := svc.ReloadTLSKeyPair()
				if err != nil {
					log.Printf("failed to reload TLS key pair: %s\n", err)
				} else {
					log.Println("successfully reloaded TLS key pair")
				}
			}

		case os.Interrupt, syscall.SIGTERM:
			log.Printf("caught %s signal; exiting\n", sig)
			err := svc.DeletePIDFile()
			if err != nil {
				log.Print(err)
			}
			os.Exit(0)

		default:
			log.Printf("caught unhandled signal %+v\n", sig)
		}
	}
}
