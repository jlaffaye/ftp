package ftp

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

type appendTLSResult struct {
	data         []byte
	handshakeErr error
}

func TestEmptyTLSAppend(t *testing.T) {
	for _, tc := range []struct {
		name, mode, value    string
		store, failHandshake bool
	}{
		{name: "implicit-empty", mode: "implicit"},
		{name: "explicit-empty", mode: "explicit"},
		{name: "plain-empty"},
		{name: "implicit-content", mode: "implicit", value: "content"},
		{name: "explicit-content", mode: "explicit", value: "content"},
		{name: "implicit-stor-control", mode: "implicit", store: true},
		{name: "explicit-stor-control", mode: "explicit", store: true},
		{name: "implicit-handshake-error", mode: "implicit", failHandshake: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			certServer := httptest.NewTLSServer(http.NotFoundHandler())
			serverTLS := certServer.TLS.Clone()
			clientTLS := certServer.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			clientTLS.ServerName = "127.0.0.1"
			certServer.Close()
			rawListener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			listener := rawListener
			if tc.mode == "implicit" {
				listener = tls.NewListener(listener, serverTLS)
			}
			done := make(chan struct{})
			results := make(chan appendTLSResult, 1)
			t.Cleanup(func() { _ = listener.Close(); <-done })
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					t.Error(err)
					return
				}
				defer func() { _ = conn.Close() }()
				if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
					t.Error(err)
					return
				}
				p := textproto.NewConn(conn)
				writeLine := func(format string, args ...interface{}) bool {
					if err := p.PrintfLine(format, args...); err != nil {
						t.Error(err)
						return false
					}
					return true
				}
				if !writeLine("220 test server") {
					return
				}
				var dataListener net.Listener
				defer func() {
					if dataListener != nil {
						_ = dataListener.Close()
					}
				}()
				for {
					line, err := p.ReadLine()
					if err != nil {
						return
					}
					command := strings.SplitN(line, " ", 2)[0]
					switch command {
					case "AUTH":
						if !writeLine("234 start TLS") {
							return
						}
						conn = tls.Server(conn, serverTLS)
						p = textproto.NewConn(conn)
					case "USER":
						if !writeLine("230 logged in") {
							return
						}
					case "FEAT":
						if !writeLine("211 no features") {
							return
						}
					case "TYPE", "PBSZ", "PROT", "NOOP":
						if !writeLine("200 OK") {
							return
						}
					case "EPSV":
						dataListener, err = net.Listen("tcp", "127.0.0.1:0")
						if err != nil {
							t.Error(err)
							return
						}
						if !writeLine("229 Entering Extended Passive Mode (|||%d|)", dataListener.Addr().(*net.TCPAddr).Port) {
							return
						}
					case "APPE", "STOR":
						if !writeLine("150 ready for data") {
							return
						}
						dc, err := dataListener.Accept()
						if err != nil {
							t.Error(err)
							return
						}
						_ = dataListener.Close()
						dataListener = nil
						if err := dc.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
							t.Error(err)
							_ = dc.Close()
							return
						}
						result := appendTLSResult{}
						if tc.mode != "" {
							conf := serverTLS.Clone()
							if tc.failHandshake {
								conf.Certificates = nil
							}
							tlsConn := tls.Server(dc, conf)
							result.handshakeErr = tlsConn.Handshake()
							dc = tlsConn
						}
						if result.handshakeErr == nil {
							result.data, err = io.ReadAll(dc)
							if err != nil {
								t.Error(err)
							}
						}
						_ = dc.Close()
						results <- result
						if result.handshakeErr != nil {
							if !writeLine("425 data TLS handshake failed") {
								return
							}
						} else {
							if !writeLine("226 transfer complete") {
								return
							}
						}
					case "QUIT":
						if !writeLine("221 bye") {
							return
						}
						return
					default:
						t.Errorf("unexpected command %q", line)
						return
					}
				}
			}()
			opts := []DialOption{DialWithTimeout(5 * time.Second), DialWithShutTimeout(5 * time.Second)}
			if tc.mode == "implicit" {
				opts = append(opts, DialWithTLS(clientTLS))
			}
			if tc.mode == "explicit" {
				opts = append(opts, DialWithExplicitTLS(clientTLS))
			}
			c, err := Dial(listener.Addr().String(), opts...)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := c.Quit(); err != nil {
					t.Error(err)
				}
			}()
			if err = c.Login("fixture", "fixture"); err != nil {
				t.Fatal(err)
			}
			if tc.store {
				err = c.Stor("empty", bytes.NewBufferString(tc.value))
			} else {
				err = c.Append("empty", bytes.NewBufferString(tc.value))
			}
			var result appendTLSResult
			select {
			case result = <-results:
			case <-time.After(5 * time.Second):
				t.Fatal("server did not observe a transfer")
			}
			if tc.failHandshake {
				if err == nil || result.handshakeErr == nil {
					t.Errorf("failed TLS handshake not returned: %v / %v", err, result.handshakeErr)
				}
			} else {
				if err != nil {
					t.Errorf("upload failed: %v", err)
				}
				if result.handshakeErr != nil {
					t.Errorf("data handshake failed: %v", result.handshakeErr)
				}
				if string(result.data) != tc.value {
					t.Errorf("data = %q, want %q", result.data, tc.value)
				}
			}
			if err = c.NoOp(); err != nil {
				t.Errorf("control connection unusable after upload: %v", err)
			}
		})
	}
}
