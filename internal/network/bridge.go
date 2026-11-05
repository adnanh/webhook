package network

import (
	"errors"
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

const frameSize = 32 * 1024

var errInvalidFrame = errors.New("invalid network data frame")

func writeFrame(conn *websocket.Conn, kind int, data []byte) error {
	_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	return conn.WriteMessage(kind, data)
}

func pingLoop(conn *websocket.Conn, stop <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				_ = conn.Close()
				return
			}
		}
	}
}

func configureDataConn(conn *websocket.Conn) {
	conn.SetReadLimit(frameSize + 16)
	_ = conn.SetReadDeadline(time.Now().Add(65 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(65 * time.Second))
	})
}

func bridgeWebSockets(a, b *websocket.Conn) {
	configureDataConn(a)
	configureDataConn(b)
	stop := make(chan struct{})
	defer close(stop)
	go pingLoop(a, stop)
	go pingLoop(b, stop)
	up := make(chan error, 1)
	go func() {
		err := copyFrames(b, a)
		if err != nil {
			_ = a.Close()
			_ = b.Close()
		}
		up <- err
	}()
	err := copyFrames(a, b)
	if err != nil {
		_ = a.Close()
		_ = b.Close()
	}
	<-up
	_ = a.Close()
	_ = b.Close()
}

func copyFrames(dst, src *websocket.Conn) error {
	for {
		kind, data, err := src.ReadMessage()
		if err != nil {
			return err
		}
		if kind == websocket.TextMessage {
			if string(data) != "FIN" {
				return errInvalidFrame
			}
			return writeFrame(dst, websocket.TextMessage, data)
		}
		if kind != websocket.BinaryMessage {
			return errInvalidFrame
		}
		if err := writeFrame(dst, websocket.BinaryMessage, data); err != nil {
			return err
		}
	}
}

func bridgeTCP(tcp net.Conn, ws *websocket.Conn) {
	defer tcp.Close()
	defer ws.Close()
	configureDataConn(ws)
	stop := make(chan struct{})
	defer close(stop)
	go pingLoop(ws, stop)
	up := make(chan error, 1)
	go func() {
		err := tcpToWebSocket(ws, tcp)
		if err != nil {
			_ = tcp.Close()
			_ = ws.Close()
		}
		up <- err
	}()
	err := webSocketToTCP(tcp, ws)
	if err != nil {
		_ = tcp.Close()
		_ = ws.Close()
	}
	<-up
}

func tcpToWebSocket(ws *websocket.Conn, tcp net.Conn) error {
	buf := make([]byte, frameSize)
	for {
		n, err := tcp.Read(buf)
		if n > 0 {
			if writeErr := writeFrame(ws, websocket.BinaryMessage, buf[:n]); writeErr != nil {
				return writeErr
			}
		}
		if err == io.EOF {
			return writeFrame(ws, websocket.TextMessage, []byte("FIN"))
		}
		if err != nil {
			return err
		}
	}
}

func webSocketToTCP(tcp net.Conn, ws *websocket.Conn) error {
	for {
		kind, data, err := ws.ReadMessage()
		if err != nil {
			return err
		}
		if kind == websocket.TextMessage {
			if string(data) != "FIN" {
				return errInvalidFrame
			}
			if closer, ok := tcp.(interface{ CloseWrite() error }); ok {
				return closer.CloseWrite()
			}
			return tcp.Close()
		}
		if kind != websocket.BinaryMessage {
			return errInvalidFrame
		}
		for len(data) != 0 {
			n, err := tcp.Write(data)
			if err != nil {
				return err
			}
			if n == 0 {
				return io.ErrShortWrite
			}
			data = data[n:]
		}
	}
}
