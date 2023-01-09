//go:build windows
// +build windows

package platform

func SetupSignals() {
	// NOOP: Windows doesn't have signals equivalent to the Unix world.
}
