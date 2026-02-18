//go:build windows
// +build windows

package main

import (
	"flag"
	"fmt"
	"github.com/Microsoft/go-winio"
	"net"
)

func platformFlags() {
	flag.StringVar(&socket, "socket", "", "path to a Windows named pipe (e.g. \\\\.\\pipe\\webhook) to use instead of listening on an ip and port; if specified, the ip and port options are ignored")
}

func trySocketListener() (net.Listener, error) {
	if socket != "" {
		addr = fmt.Sprintf("{pipe:%s}", socket)
		return winio.ListenPipe(socket, nil)
	}
	return nil, nil
}
