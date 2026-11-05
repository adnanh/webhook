package network

import (
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// pipePair preserves TCP half-close semantics across a relay hop.
func pipePair() (net.Conn, net.Conn) {
	aRead, bWrite := io.Pipe()
	bRead, aWrite := io.Pipe()
	return &pipeConn{read: aRead, write: aWrite}, &pipeConn{read: bRead, write: bWrite}
}

type pipeConn struct {
	read            *io.PipeReader
	write           *io.PipeWriter
	mu              sync.Mutex
	readTimer       *time.Timer
	writeTimer      *time.Timer
	readGeneration  uint64
	writeGeneration uint64
	readExpired     bool
	writeExpired    bool
}

func (c *pipeConn) Read(p []byte) (int, error) {
	n, err := c.read.Read(p)
	if err != nil {
		c.mu.Lock()
		if c.readExpired {
			err = os.ErrDeadlineExceeded
		}
		c.mu.Unlock()
	}
	return n, err
}
func (c *pipeConn) Write(p []byte) (int, error) {
	n, err := c.write.Write(p)
	if err != nil {
		c.mu.Lock()
		if c.writeExpired {
			err = os.ErrDeadlineExceeded
		}
		c.mu.Unlock()
	}
	return n, err
}
func (c *pipeConn) CloseWrite() error {
	_ = c.SetWriteDeadline(time.Time{})
	return c.write.Close()
}
func (c *pipeConn) CloseRead() error {
	_ = c.SetReadDeadline(time.Time{})
	return c.read.Close()
}
func (c *pipeConn) Close() error {
	_ = c.SetDeadline(time.Time{})
	_ = c.read.Close()
	return c.write.Close()
}
func (c *pipeConn) LocalAddr() net.Addr  { return pipeAddr{} }
func (c *pipeConn) RemoteAddr() net.Addr { return pipeAddr{} }
func (c *pipeConn) SetDeadline(t time.Time) error {
	_ = c.SetReadDeadline(t)
	return c.SetWriteDeadline(t)
}
func (c *pipeConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readGeneration++
	c.readExpired = false
	if c.readTimer != nil {
		c.readTimer.Stop()
	}
	if !t.IsZero() {
		generation := c.readGeneration
		c.readTimer = time.AfterFunc(time.Until(t), func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			if c.readGeneration == generation {
				c.readExpired = true
				_ = c.read.CloseWithError(os.ErrDeadlineExceeded)
			}
		})
	}
	return nil
}
func (c *pipeConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeGeneration++
	c.writeExpired = false
	if c.writeTimer != nil {
		c.writeTimer.Stop()
	}
	if !t.IsZero() {
		generation := c.writeGeneration
		c.writeTimer = time.AfterFunc(time.Until(t), func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			if c.writeGeneration == generation {
				c.writeExpired = true
				_ = c.write.CloseWithError(os.ErrDeadlineExceeded)
			}
		})
	}
	return nil
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "network relay" }
