//go:build !windows
// +build !windows

package main

import (
	"flag"
	"fmt"
	"github.com/coreos/go-systemd/v22/activation"
	"net"
)

func platformFlags() {
	flag.StringVar(&socket, "socket", "", "path to a Unix socket (e.g. /tmp/webhook.sock) to use instead of listening on an ip and port; if specified, the ip and port options are ignored")
	flag.IntVar(&setGID, "setgid", 0, "set group ID after opening listening port; must be used with setuid, not permitted with -socket")
	flag.IntVar(&setUID, "setuid", 0, "set user ID after opening listening port; must be used with setgid, not permitted with -socket")
}

func trySocketListener() (net.Listener, error) {
	// first check whether we have any sockets from systemd
	listeners, err := activation.Listeners()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve sockets from systemd: %w", err)
	}
	numListeners := len(listeners)
	if numListeners > 1 {
		return nil, fmt.Errorf("received %d sockets from systemd, but only 1 is supported", numListeners)
	}
	if numListeners == 1 {
		sockAddr := listeners[0].Addr()
		if sockAddr.Network() == "tcp" {
			addr = sockAddr.String()
		} else {
			addr = fmt.Sprintf("{%s:%s}", sockAddr.Network(), sockAddr.String())
		}
		return listeners[0], nil
	}
	// if we get to here, we got no sockets from systemd, so check -socket flag
	if socket != "" {
		if setGID != 0 || setUID != 0 {
			return nil, fmt.Errorf("-setuid and -setgid options are not compatible with -socket.  If you need to bind a socket as root but run webhook as a different user, consider using systemd activation")
		}
		addr = fmt.Sprintf("{unix:%s}", socket)
		return net.Listen("unix", socket)
	}
	return nil, nil
}
