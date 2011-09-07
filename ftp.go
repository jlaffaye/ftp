package ftp

import (
	"bufio"
	"io"
	"net"
	"net/textproto"
	"os"
	"fmt"
	"strconv"
	"strings"
)

type EntryType int

const (
	EntryTypeFile EntryType = iota
	EntryTypeFolder
	EntryTypeLink
)

type ServerConn struct {
	conn *textproto.Conn
	host string
}

type Entry struct {
	Name string
	Type EntryType
	Size uint64
}

type response struct {
	conn net.Conn
	c    *ServerConn
}

// Connect to a ftp server and returns a ServerConn handler.
func Connect(addr string) (*ServerConn, os.Error) {
	conn, err := textproto.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	a := strings.SplitN(addr, ":", 2)
	c := &ServerConn{conn, a[0]}

	_, _, err = c.conn.ReadCodeLine(StatusReady)
	if err != nil {
		c.Quit()
		return nil, err
	}

	return c, nil
}

func (c *ServerConn) Login(user, password string) os.Error {
	_, _, err := c.cmd(StatusUserOK, "USER %s", user)
	if err != nil {
		return err
	}

	_, _, err = c.cmd(StatusLoggedIn, "PASS %s", password)
	return err
}

// Enter extended passive mode
func (c *ServerConn) epsv() (port int, err os.Error) {
	c.conn.Cmd("EPSV")
	_, line, err := c.conn.ReadCodeLine(StatusExtendedPassiveMode)
	if err != nil {
		return
	}
	start := strings.Index(line, "|||")
	end := strings.LastIndex(line, "|")
	if start == -1 || end == -1 {
		err = os.NewError("Invalid EPSV response format")
		return
	}
	port, err = strconv.Atoi(line[start+3 : end])
	return
}

// Open a new data connection using extended passive mode
func (c *ServerConn) openDataConn() (net.Conn, os.Error) {
	port, err := c.epsv()
	if err != nil {
		return nil, err
	}

	// Build the new net address string
	addr := fmt.Sprintf("%s:%d", c.host, port)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// Helper function to execute a command and check for the expected code
func (c *ServerConn) cmd(expected int, format string, args ...interface{}) (int, string, os.Error) {
	_, err := c.conn.Cmd(format, args...)
	if err != nil {
		return 0, "", err
	}

	code, line, err := c.conn.ReadCodeLine(expected)
	return code, line, err
}

// Helper function to execute commands which require a data connection
func (c *ServerConn) cmdDataConn(format string, args ...interface{}) (net.Conn, os.Error) {
	conn, err := c.openDataConn()
	if err != nil {
		return nil, err
	}

	_, err = c.conn.Cmd(format, args...)
	if err != nil {
		conn.Close()
		return nil, err
	}

	code, msg, err := c.conn.ReadCodeLine(-1)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if code != StatusAlreadyOpen && code != StatusAboutToSend {
		conn.Close()
		return nil, &textproto.Error{code, msg}
	}

	return conn, nil
}

func parseListLine(line string) (*Entry, os.Error) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return nil, os.NewError("Unsupported LIST line")
	}

	e := &Entry{}
	switch fields[0][0] {
	case '-':
		e.Type = EntryTypeFile
	case 'd':
		e.Type = EntryTypeFolder
	case 'l':
		e.Type = EntryTypeLink
	default:
		return nil, os.NewError("Unknown entry type")
	}

	e.Name = strings.Join(fields[8:], " ")
	return e, nil
}

func (c *ServerConn) List(path string) (entries []*Entry, err os.Error) {
	conn, err := c.cmdDataConn("LIST %s", path)
	if err != nil {
		return
	}

	r := &response{conn, c}
	defer r.Close()

	bio := bufio.NewReader(r)
	for {
		line, e := bio.ReadString('\n')
		if e == os.EOF {
			break
		} else if e != nil {
			return nil, e
		}
		entry, err := parseListLine(line)
		if err == nil {
			entries = append(entries, entry)
		}
	}
	return
}

func (c *ServerConn) ChangeDir(path string) os.Error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "CWD %s", path)
	return err
}

func (c *ServerConn) ChangeDirToParent() os.Error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "CDUP")
	return err
}

func (c *ServerConn) CurrentDir() (string, os.Error) {
	_, msg, err := c.cmd(StatusPathCreated, "PWD")
	if err != nil {
		return "", err
	}

	start := strings.Index(msg, "\"")
	end := strings.LastIndex(msg, "\"")

	if start == -1 || end == -1 {
		return "", os.NewError("Unsuported PWD response format")
	}

	return msg[start+1:end], nil
}

// Retrieves a remote file
func (c *ServerConn) Retr(path string) (io.ReadCloser, os.Error) {
	conn, err := c.cmdDataConn("RETR %s", path)
	if err != nil {
		return nil, err
	}

	r := &response{conn, c}
	return r, nil
}

func (c *ServerConn) Stor(name string, r io.Reader) os.Error {
	conn, err := c.cmdDataConn("STOR %s", name)
	if err != nil {
		return err
	}

	_, err = io.Copy(conn, r)
	conn.Close()
	if err != nil {
		return err
	}

	_, _, err = c.conn.ReadCodeLine(StatusClosingDataConnection)
	return err
}

func (c *ServerConn) Rename(from, to string) os.Error {
	_, _, err := c.cmd(StatusRequestFilePending, "RNFR %s", from)
	if err != nil {
		return err
	}

	_, _, err = c.cmd(StatusRequestedFileActionOK, "RNTO %s", to)
	return err
}

func (c *ServerConn) Delete(name string) os.Error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "DELE %s", name)
	return err
}

func (c *ServerConn) MakeDir(name string) os.Error {
	_, _, err := c.cmd(StatusPathCreated, "MKD %s", name)
	return err
}

func (c *ServerConn) RemoveDir(name string) os.Error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "RMD %s", name)
	return err
}

// Sends a NOOP command. Usualy used to prevent timeouts.
func (c *ServerConn) NoOp() os.Error {
	_, _, err := c.cmd(StatusCommandOK, "NOOP")
	return err
}

func (c *ServerConn) Quit() os.Error {
	c.conn.Cmd("QUIT")
	return c.conn.Close()
}

func (r *response) Read(buf []byte) (int, os.Error) {
	n, err := r.conn.Read(buf)
	if err == os.EOF {
		_, _, err2 := r.c.conn.ReadCodeLine(StatusClosingDataConnection)
		if err2 != nil {
			err = err2
		}
	}
	return n, err
}

func (r *response) Close() os.Error {
	return r.conn.Close()
}
