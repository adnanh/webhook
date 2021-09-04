// +build windows

package main

import "github.com/adnanh/webhook/internal/service"

func setupSignals(_ *service.Service) {
	// NOOP: Windows doesn't have signals equivalent to the Unix world.
}
