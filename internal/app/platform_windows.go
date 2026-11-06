//go:build windows
// +build windows

package app

import (
	"flag"
	"fmt"
	"github.com/Microsoft/go-winio"
	"net"
)

func platformFlags() {
	flag.StringVar(&socket, "socket", "", `Windows 命名管道路径（例如 \\.\pipe\webhook）；指定后忽略 ip 和 port 参数`)
}

func trySocketListener() (net.Listener, error) {
	if socket != "" {
		addr = fmt.Sprintf("{pipe:%s}", socket)
		return winio.ListenPipe(socket, nil)
	}
	return nil, nil
}
