//go:build !windows
// +build !windows

package app

import (
	"flag"
	"fmt"
	"github.com/coreos/go-systemd/v22/activation"
	"net"
)

func platformFlags() {
	flag.StringVar(&socket, "socket", "", "Unix socket 路径（例如 /tmp/webhook.sock）；指定后忽略 ip 和 port 参数")
	flag.IntVar(&setGID, "setgid", 0, "打开监听端口后切换的用户组 ID；须与 setuid 同时使用，不可与 socket 同时使用")
	flag.IntVar(&setUID, "setuid", 0, "打开监听端口后切换的用户 ID；须与 setgid 同时使用，不可与 socket 同时使用")
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
