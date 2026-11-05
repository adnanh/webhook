package network

import (
	"net"
	"net/http"
	"time"
)

// serveWebhookInProcess connects a tunnel directly to the webhook handler.
// No TCP or Unix listener is created on the agent host.
func serveWebhookInProcess(handler http.Handler) net.Conn {
	client, server := net.Pipe()
	go func() {
		listener := &oneConnListener{conn: loopbackConn{Conn: server}}
		_ = (&http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}).Serve(listener)
	}()
	return client
}

type oneConnListener struct {
	conn net.Conn
}

func (l *oneConnListener) Accept() (net.Conn, error) {
	if l.conn == nil {
		return nil, net.ErrClosed
	}
	conn := l.conn
	l.conn = nil
	return conn, nil
}

func (l *oneConnListener) Close() error   { return nil }
func (l *oneConnListener) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }

type loopbackConn struct{ net.Conn }

func (loopbackConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}
}
