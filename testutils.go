//go:build !windows
// +build !windows

package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"path"
)

func prepareTestSocket(_ string) (socketPath string, transport *http.Transport, cleanup func(), err error) {
	tmp, err := os.MkdirTemp("", "webhook-socket-")
	if err != nil {
		return "", nil, nil, err
	}
	cleanup = func() { os.RemoveAll(tmp) }
	socketPath = path.Join(tmp, "webhook.sock")
	socketDialer := &net.Dialer{}
	transport = &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return socketDialer.DialContext(ctx, "unix", socketPath)
		},
	}

	return socketPath, transport, cleanup, nil
}
