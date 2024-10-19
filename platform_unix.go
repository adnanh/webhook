//go:build !windows
// +build !windows

package main

import (
	"flag"
	"fmt"
	"net"
)

func platformFlags() {
	flag.StringVar(&socket, "socket", "", "path to a Unix socket (e.g. /tmp/webhook.sock) to use instead of listening on an ip and port; if specified, the ip and port options are ignored")
}

func trySocketListener() (net.Listener, error) {
	if socket != "" {
		addr = fmt.Sprintf("{unix:%s}", socket)
		return net.Listen("unix", socket)
	}
	return nil, nil
}
