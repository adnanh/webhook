//go:build windows
// +build windows

package app

func setupSignals() {
	// NOOP: Windows doesn't have signals equivalent to the Unix world.
}
