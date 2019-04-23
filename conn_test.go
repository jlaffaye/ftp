package ftp

import (
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/textproto"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type ftpMock struct {
	address  string
	listener *net.TCPListener
	proto    *textproto.Conn
	commands []string // list of received commands
	rest     int
	dataConn *mockDataConn
	sync.WaitGroup
}

// newFtpMock returns a mock implementation of a FTP server
// For simplication, a mock instance only accepts a signle connection and terminates afer
func newFtpMock(t *testing.T, address string) (*ftpMock, error) {
	var err error
	mock := &ftpMock{address: address}

	l, err := net.Listen("tcp", address+":0")
	if err != nil {
		return nil, err
	}

	tcpListener, ok := l.(*net.TCPListener)
	if !ok {
		return nil, errors.New("listener is not a net.TCPListener")
	}
	mock.listener = tcpListener

	go mock.listen(t)

	return mock, nil
}

func (mock *ftpMock) listen(t *testing.T) {
	// Listen for an incoming connection.
	conn, err := mock.listener.Accept()
	if err != nil {
		t.Errorf("can not accept: %s", err)
		return
	}

	// Do not accept incoming connections anymore
	mock.listener.Close()

	mock.Add(1)
	defer mock.Done()
	defer conn.Close()

	mock.proto = textproto.NewConn(conn)
	mock.proto.Writer.PrintfLine("220 FTP Server ready.")

	for {
		fullCommand, _ := mock.proto.ReadLine()

		cmdParts := strings.Split(fullCommand, " ")

		// Append to list of received commands
		mock.commands = append(mock.commands, cmdParts[0])

		// At least one command must have a multiline response
		switch cmdParts[0] {
		case "FEAT":
			mock.proto.Writer.PrintfLine("211-Features:\r\n FEAT\r\n PASV\r\n EPSV\r\n SIZE\r\n211 End")
		case "USER":
			if cmdParts[1] == "anonymous" {
				mock.proto.Writer.PrintfLine("331 Please send your password")
			} else {
				mock.proto.Writer.PrintfLine("530 This FTP server is anonymous only")
			}
		case "PASS":
			mock.proto.Writer.PrintfLine("230-Hey,\r\nWelcome to my FTP\r\n230 Access granted")
		case "TYPE":
			mock.proto.Writer.PrintfLine("200 Type set ok")
		case "CWD":
			if cmdParts[1] == "missing-dir" {
				mock.proto.Writer.PrintfLine("550 %s: No such file or directory", cmdParts[1])
			} else {
				mock.proto.Writer.PrintfLine("250 Directory successfully changed.")
			}
		case "DELE":
			mock.proto.Writer.PrintfLine("250 File successfully removed.")
		case "MKD":
			mock.proto.Writer.PrintfLine("257 Directory successfully created.")
		case "RMD":
			if cmdParts[1] == "missing-dir" {
				mock.proto.Writer.PrintfLine("550 No such file or directory")
			} else {
				mock.proto.Writer.PrintfLine("250 Directory successfully removed.")
			}
		case "PWD":
			mock.proto.Writer.PrintfLine("257 \"/incoming\"")
		case "CDUP":
			mock.proto.Writer.PrintfLine("250 CDUP command successful")
		case "SIZE":
			if cmdParts[1] == "magic-file" {
				mock.proto.Writer.PrintfLine("213 42")
			} else {
				mock.proto.Writer.PrintfLine("550 Could not get file size.")
			}
		case "PASV":
			p, err := mock.listenDataConn()
			if err != nil {
				mock.proto.Writer.PrintfLine("451 %s.", err)
				break
			}

			p1 := int(p / 256)
			p2 := p % 256

			mock.proto.Writer.PrintfLine("227 Entering Passive Mode (127,0,0,1,%d,%d).", p1, p2)
		case "EPSV":
			p, err := mock.listenDataConn()
			if err != nil {
				mock.proto.Writer.PrintfLine("451 %s.", err)
				break
			}
			mock.proto.Writer.PrintfLine("229 Entering Extended Passive Mode (|||%d|)", p)
		case "STOR":
			if mock.dataConn == nil {
				mock.proto.Writer.PrintfLine("425 Unable to build data connection: Connection refused")
				break
			}
			mock.proto.Writer.PrintfLine("150 please send")
			mock.recvDataConn()
		case "LIST":
			if mock.dataConn == nil {
				mock.proto.Writer.PrintfLine("425 Unable to build data connection: Connection refused")
				break
			}

			mock.dataConn.Wait()
			mock.proto.Writer.PrintfLine("150 Opening ASCII mode data connection for file list")
			mock.dataConn.conn.Write([]byte("-rw-r--r--   1 ftp      wheel           0 Jan 29 10:29 lo"))
			mock.proto.Writer.PrintfLine("226 Transfer complete")
			mock.closeDataConn()
		case "NLST":
			if mock.dataConn == nil {
				mock.proto.Writer.PrintfLine("425 Unable to build data connection: Connection refused")
				break
			}

			mock.dataConn.Wait()
			mock.proto.Writer.PrintfLine("150 Opening ASCII mode data connection for file list")
			mock.dataConn.conn.Write([]byte("/incoming"))
			mock.proto.Writer.PrintfLine("226 Transfer complete")
			mock.closeDataConn()
		case "RETR":
			if mock.dataConn == nil {
				mock.proto.Writer.PrintfLine("425 Unable to build data connection: Connection refused")
				break
			}

			mock.dataConn.Wait()
			mock.proto.Writer.PrintfLine("150 Opening ASCII mode data connection for file list")
			mock.dataConn.conn.Write([]byte(testData[mock.rest:]))
			mock.rest = 0
			mock.proto.Writer.PrintfLine("226 Transfer complete")
			mock.closeDataConn()
		case "RNFR":
			mock.proto.Writer.PrintfLine("350 File or directory exists, ready for destination name")
		case "RNTO":
			mock.proto.Writer.PrintfLine("250 Rename successful")
		case "REST":
			if len(cmdParts) != 2 {
				mock.proto.Writer.PrintfLine("500 wrong number of arguments")
				break
			}
			rest, err := strconv.Atoi(cmdParts[1])
			if err != nil {
				mock.proto.Writer.PrintfLine("500 REST: %s", err)
				break
			}
			mock.rest = rest
			mock.proto.Writer.PrintfLine("350 Restarting at %s. Send STORE or RETRIEVE to initiate transfer", cmdParts[1])
		case "NOOP":
			mock.proto.Writer.PrintfLine("200 NOOP ok.")
		case "REIN":
			mock.proto.Writer.PrintfLine("220 Logged out")
		case "QUIT":
			mock.proto.Writer.PrintfLine("221 Goodbye.")
			return
		default:
			mock.proto.Writer.PrintfLine("500 Unknown command %s.", cmdParts[0])
		}
	}
}

func (mock *ftpMock) closeDataConn() (err error) {
	if mock.dataConn != nil {
		err = mock.dataConn.Close()
		mock.dataConn = nil
	}
	return
}

type mockDataConn struct {
	listener *net.TCPListener
	conn     net.Conn
	// WaitGroup is done when conn is accepted and stored
	sync.WaitGroup
}

func (d *mockDataConn) Close() (err error) {
	if d.listener != nil {
		err = d.listener.Close()
	}
	if d.conn != nil {
		err = d.conn.Close()
	}
	return
}

func (mock *ftpMock) listenDataConn() (int64, error) {
	mock.closeDataConn()

	l, err := net.Listen("tcp", mock.address+":0")
	if err != nil {
		return 0, err
	}

	tcpListener, ok := l.(*net.TCPListener)
	if !ok {
		return 0, errors.New("listener is not a net.TCPListener")
	}

	addr := tcpListener.Addr().String()

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}

	p, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return 0, err
	}

	dataConn := &mockDataConn{listener: tcpListener}
	dataConn.Add(1)

	go func() {
		// Listen for an incoming connection.
		conn, err := dataConn.listener.Accept()
		if err != nil {
			// t.Errorf("can not accept: %s", err)
			return
		}

		dataConn.conn = conn
		dataConn.Done()
	}()

	mock.dataConn = dataConn
	return p, nil
}

func (mock *ftpMock) recvDataConn() {
	mock.dataConn.Wait()
	io.Copy(ioutil.Discard, mock.dataConn.conn)
	mock.proto.Writer.PrintfLine("226 Transfer Complete")
	mock.closeDataConn()
}

func (mock *ftpMock) Addr() string {
	return mock.listener.Addr().String()
}

// Closes the listening socket
func (mock *ftpMock) Close() {
	mock.listener.Close()
}

// Helper to return a client connected to a mock server
func openConn(t *testing.T, addr string, options ...DialOption) (*ftpMock, *ServerConn) {
	mock, err := newFtpMock(t, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	c, err := Dial(mock.Addr(), options...)
	if err != nil {
		t.Fatal(err)
	}

	err = c.Login("anonymous", "anonymous")
	if err != nil {
		t.Fatal(err)
	}

	return mock, c

}

// Helper to close a client connected to a mock server
func closeConn(t *testing.T, mock *ftpMock, c *ServerConn, commands []string) {
	expected := []string{"FEAT", "USER", "PASS", "TYPE"}
	expected = append(expected, commands...)
	expected = append(expected, "QUIT")

	if err := c.Quit(); err != nil {
		t.Fatal(err)
	}

	// Wait for the connection to close
	mock.Wait()

	if !reflect.DeepEqual(mock.commands, expected) {
		t.Fatal("unexpected sequence of commands:", mock.commands, "expected:", expected)
	}
}

func TestConn4(t *testing.T) {
	mock, c := openConn(t, "127.0.0.1")
	closeConn(t, mock, c, nil)
}

func TestConn6(t *testing.T) {
	mock, c := openConn(t, "[::1]")
	closeConn(t, mock, c, nil)
}
