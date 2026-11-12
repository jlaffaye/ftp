package ftp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const dialTLSTestTimeout = 5 * time.Second

type dialTLSTestConn struct {
	net.Conn
	closes atomic.Int32
}

func (c *dialTLSTestConn) Close() error {
	c.closes.Add(1)
	return c.Conn.Close()
}

func TestDialWithDialFuncTLSList(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		rejectData bool
	}{
		{name: "explicit", mode: "explicit"},
		{name: "plain"},
		{name: "implicit", mode: "implicit"},
		{name: "explicit-verification-error", mode: "explicit", rejectData: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			certServer := httptest.NewTLSServer(http.NotFoundHandler())
			serverTLS := certServer.TLS.Clone()
			clientTLS := certServer.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
			clientTLS.ServerName = "127.0.0.1"
			certServer.Close()

			verificationErr := errors.New("data certificate rejected")
			var verified atomic.Int32
			clientTLS.VerifyConnection = func(state tls.ConnectionState) error {
				if len(state.VerifiedChains) == 0 {
					return errors.New("peer certificate was not verified")
				}
				if verified.Add(1) == 2 && tc.rejectData {
					return verificationErr
				}
				return nil
			}

			control, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
			require.NoError(t, err)
			t.Cleanup(func() { _ = control.Close() })
			data, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
			require.NoError(t, err)
			t.Cleanup(func() { _ = data.Close() })
			require.NoError(t, control.SetDeadline(time.Now().Add(dialTLSTestTimeout)))
			require.NoError(t, data.SetDeadline(time.Now().Add(dialTLSTestTimeout)))

			done := make(chan error, 1)
			var connections []*dialTLSTestConn
			t.Cleanup(func() {
				_ = control.Close()
				_ = data.Close()
				for _, conn := range connections {
					_ = conn.Close()
				}
				select {
				case err := <-done:
					if !t.Failed() {
						require.NoError(t, err)
					}
				case <-time.After(dialTLSTestTimeout):
					t.Error("FTP fixture did not stop")
				}
			})
			go func() { done <- serveDialTLSList(control, data, serverTLS, tc.mode) }()

			dial := func(network, address string) (net.Conn, error) {
				conn, err := net.DialTimeout(network, address, dialTLSTestTimeout)
				if err != nil {
					return nil, err
				}
				if err := conn.SetDeadline(time.Now().Add(dialTLSTestTimeout)); err != nil {
					_ = conn.Close()
					return nil, err
				}
				tracked := &dialTLSTestConn{Conn: conn}
				connections = append(connections, tracked)
				if tc.mode == "implicit" {
					return tls.Client(tracked, clientTLS), nil
				}
				return tracked, nil
			}
			opts := []DialOption{DialWithDialFunc(dial), DialWithShutTimeout(dialTLSTestTimeout)}
			switch tc.mode {
			case "explicit":
				opts = append(opts, DialWithExplicitTLS(clientTLS))
			case "implicit":
				opts = append(opts, DialWithTLS(clientTLS))
			}
			c, err := Dial(control.Addr().String(), opts...)
			require.NoError(t, err)
			require.NoError(t, c.Login("anonymous", "anonymous"))

			entries, err := c.List("/")
			if tc.rejectData {
				require.ErrorIs(t, err, verificationErr)
			} else {
				require.NoError(t, err)
				require.Len(t, entries, 1)
				require.Equal(t, "report.txt", entries[0].Name)
				require.Equal(t, uint64(5), entries[0].Size)
			}
			if tc.mode != "" {
				require.Equal(t, int32(2), verified.Load())
			}
			require.Len(t, connections, 2)
			require.GreaterOrEqual(t, connections[1].closes.Load(), int32(1))
			require.NoError(t, c.NoOp())
			require.NoError(t, c.Quit())
			require.GreaterOrEqual(t, connections[0].closes.Load(), int32(1))
		})
	}
}

func serveDialTLSList(control, data *net.TCPListener, config *tls.Config, mode string) error {
	raw, err := control.AcceptTCP()
	if err != nil {
		return err
	}
	defer func() { _ = raw.Close() }()
	if err := raw.SetDeadline(time.Now().Add(dialTLSTestTimeout)); err != nil {
		return err
	}
	var conn net.Conn = raw
	if mode == "implicit" {
		conn = tls.Server(raw, config)
	}
	p := textproto.NewConn(conn)
	if err := p.PrintfLine("220 FTP fixture"); err != nil {
		return err
	}
	exchange := func(command, reply string) error {
		line, err := p.ReadLine()
		if err != nil {
			return err
		}
		if line != command {
			return fmt.Errorf("got command %q, want %q", line, command)
		}
		if reply == "" {
			return nil
		}
		return p.PrintfLine("%s", reply)
	}
	if mode == "explicit" {
		if err := exchange("AUTH TLS", "234 Begin TLS"); err != nil {
			return err
		}
		conn = tls.Server(raw, config)
		p = textproto.NewConn(conn)
	}
	for _, step := range [][2]string{{"USER anonymous", "230 Logged in"}, {"FEAT", "211 End"}, {"TYPE I", "200 OK"}} {
		if err := exchange(step[0], step[1]); err != nil {
			return err
		}
	}
	if mode != "" {
		for _, command := range []string{"PBSZ 0", "PROT P"} {
			if err := exchange(command, "200 OK"); err != nil {
				return err
			}
		}
	}
	if err := exchange("EPSV", fmt.Sprintf("229 Entering Extended Passive Mode (|||%d|)", data.Addr().(*net.TCPAddr).Port)); err != nil {
		return err
	}
	rawData, err := data.AcceptTCP()
	if err != nil {
		return err
	}
	defer func() { _ = rawData.Close() }()
	if err := rawData.SetDeadline(time.Now().Add(dialTLSTestTimeout)); err != nil {
		return err
	}
	if err := exchange("LIST /", "150 Sending listing"); err != nil {
		return err
	}
	var dataConn net.Conn = rawData
	if mode != "" {
		dataConn = tls.Server(rawData, config)
	}
	_, transferErr := io.WriteString(dataConn, "-rw-r--r-- 1 owner group 5 Jan 2 2024 report.txt\r\n")
	_ = dataConn.Close()
	if transferErr != nil {
		err = p.PrintfLine("426 Data transfer failed")
	} else {
		err = p.PrintfLine("226 Transfer complete")
	}
	if err != nil {
		return err
	}
	if err := exchange("NOOP", "200 OK"); err != nil {
		return err
	}
	if err := exchange("QUIT", ""); err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, conn)
	return err
}
