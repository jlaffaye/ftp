package ftp

import (
	"net"
	"net/textproto"
	"testing"
	"time"
)

type completionConn struct {
	net.Conn
	deadline time.Time
	closed   bool
}

func (c *completionConn) SetDeadline(deadline time.Time) error {
	c.deadline = deadline
	return c.Conn.SetDeadline(deadline)
}

func (c *completionConn) Close() error {
	c.closed = true
	return c.Conn.Close()
}

func TestCompletionDeadlineIsCleared(t *testing.T) {
	control, peer := net.Pipe()
	defer peer.Close()
	conn := &completionConn{Conn: control}
	defer conn.Close()
	client := &ServerConn{netConn: conn, conn: textproto.NewConn(conn), options: &dialOptions{shutTimeout: time.Second}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = peer.Write([]byte("226 complete\r\n"))
	}()
	if err := client.checkDataShut(); err != nil {
		t.Fatal(err)
	}
	<-done
	if !conn.deadline.IsZero() || conn.closed {
		t.Fatalf("deadline = %v, closed = %v", conn.deadline, conn.closed)
	}
}

func TestCompletionFailureClosesControl(t *testing.T) {
	for _, response := range []string{"", "426 transfer aborted\r\n"} {
		t.Run(response, func(t *testing.T) {
			control, peer := net.Pipe()
			defer peer.Close()
			conn := &completionConn{Conn: control}
			defer conn.Close()
			client := &ServerConn{netConn: conn, conn: textproto.NewConn(conn), options: &dialOptions{shutTimeout: 100 * time.Millisecond}}
			done := make(chan struct{})
			go func() {
				defer close(done)
				if response != "" {
					_, _ = peer.Write([]byte(response))
				}
			}()
			if err := client.checkDataShut(); err == nil {
				t.Fatal("expected completion error")
			}
			<-done
			if !conn.closed || !conn.deadline.IsZero() {
				t.Fatalf("closed = %v, deadline = %v", conn.closed, conn.deadline)
			}
		})
	}
}
