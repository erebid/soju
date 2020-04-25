package soju

import (
	"fmt"
	"net"

	"gopkg.in/irc.v3"
)

func setKeepAlive(c net.Conn) error {
	tcpConn, ok := c.(*net.TCPConn)
	if !ok {
		return fmt.Errorf("cannot enable keep-alive on a non-TCP connection")
	}
	if err := tcpConn.SetKeepAlive(true); err != nil {
		return err
	}
	return tcpConn.SetKeepAlivePeriod(keepAlivePeriod)
}

type conn struct {
	net      net.Conn
	irc      *irc.Conn
	srv      *Server
	logger   Logger
	outgoing chan<- *irc.Message
	closed   chan struct{}
}

func newConn(srv *Server, netConn net.Conn, logger Logger) *conn {
	setKeepAlive(netConn)

	outgoing := make(chan *irc.Message, 64)
	c := &conn{
		net:      netConn,
		irc:      irc.NewConn(netConn),
		srv:      srv,
		outgoing: outgoing,
		logger:   logger,
		closed:   make(chan struct{}),
	}

	go func() {
		for msg := range outgoing {
			if c.srv.Debug {
				c.logger.Printf("sent: %v", msg)
			}
			// TODO: https://github.com/nhooyr/websocket/issues/228
			//c.net.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.irc.WriteMessage(msg); err != nil {
				c.logger.Printf("failed to write message: %v", err)
				break
			}
		}
		if err := c.net.Close(); err != nil {
			c.logger.Printf("failed to close connection: %v", err)
		} else {
			c.logger.Printf("connection closed")
		}
		// Drain the outgoing channel to prevent SendMessage from blocking
		for range outgoing {
			// This space is intentionally left blank
		}
	}()

	c.logger.Printf("new connection")
	return c
}

func (c *conn) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

// Close closes the connection. It is safe to call from any goroutine.
func (c *conn) Close() error {
	if c.isClosed() {
		return fmt.Errorf("connection already closed")
	}
	close(c.closed)
	close(c.outgoing)
	return nil
}

func (c *conn) ReadMessage() (*irc.Message, error) {
	msg, err := c.irc.ReadMessage()
	if err != nil {
		return nil, err
	}

	if c.srv.Debug {
		c.logger.Printf("received: %v", msg)
	}

	return msg, nil
}

// SendMessage queues a new outgoing message. It is safe to call from any
// goroutine.
func (c *conn) SendMessage(msg *irc.Message) {
	if c.isClosed() {
		return
	}
	c.outgoing <- msg
}
