//go:build windows
// +build windows

package main

import (
	"context"
	"github.com/Microsoft/go-winio"
	"net"
	"net/http"
)

func prepareTestSocket(hookTmpl string) (socketPath string, transport *http.Transport, cleanup func(), err error) {
	socketPath = "\\\\.\\pipe\\webhook-" + hookTmpl
	transport = &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return winio.DialPipeContext(ctx, socketPath)
		},
	}

	return socketPath, transport, nil, nil
}
