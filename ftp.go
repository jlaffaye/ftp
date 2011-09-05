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
func Connect(host, user, password string) (*ServerConn, os.Error) {
	conn, err := textproto.Dial("tcp", host)
	if err != nil {
		return nil, err
	}

	a := strings.SplitN(host, ":", 2)
	c := &ServerConn{conn, a[0]}

	_, _, err = c.conn.ReadCodeLine(StatusReady)
	if err != nil {
		c.Quit()
		return nil, err
	}

	c.conn.Cmd("USER %s", user)
	_, _, err = c.conn.ReadCodeLine(StatusUserOK)
	if err != nil {
		c.Quit()
		return nil, err
	}

	c.conn.Cmd("PASS %s", password)
	_, _, err = c.conn.ReadCodeLine(StatusLoggedIn)
	if err != nil {
		c.Quit()
		return nil, err
	}

	return c, nil
}

// Like Connect() but with anonymous credentials.
func ConnectAnonymous(host string) (*ServerConn, os.Error) {
	return Connect(host, "anonymous", "anonymous")
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
func (c *ServerConn) openDataConnection() (net.Conn, os.Error) {
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

// Helper function to check if the last command succeeded and if it will
// send the data to the data connection.
// This is needed because some servers return StatusAboutToSend (150)
// and some StatusAlreadyOpen (125)
func (c *ServerConn) checkDataConn() os.Error {
	code, msg, err := c.conn.ReadCodeLine(-1)
	if err != nil {
		return err
	}
	if code != StatusAlreadyOpen && code != StatusAboutToSend {
		return os.NewError(fmt.Sprintf("%d %s", code, msg))
	}
	return nil
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
	conn, err := c.openDataConnection()
	if err != nil {
		return
	}

	r := &response{conn, c}
	defer r.Close()

	_, err = c.conn.Cmd("LIST %s", path)
	if err != nil {
		return
	}

	err = c.checkDataConn()
	if err != nil {
		return
	}

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

func (c *ServerConn) ChangeDir(path string) (err os.Error) {
	_, err = c.conn.Cmd("CWD %s", path)
	if err == nil {
		_, _, err = c.conn.ReadCodeLine(StatusRequestedFileActionOK)
	}
	return
}

// Retrieves a remote file
func (c *ServerConn) Retr(path string) (io.ReadCloser, os.Error) {
	conn, err := c.openDataConnection()
	if err != nil {
		return nil, err
	}

	_, err = c.conn.Cmd("RETR %s", path)
	if err != nil {
		conn.Close()
		return nil, err
	}

	err = c.checkDataConn()
	if err != nil {
		conn.Close()
		return nil, err
	}

	r := &response{conn, c}
	return r, nil
}

func (c *ServerConn) Stor(name string, r io.Reader) os.Error {
	conn, err := c.openDataConnection()
	if err != nil {
		return err
	}

	_, err = c.conn.Cmd("STOR %s", name)
	if err != nil {
		conn.Close()
		return err
	}

	err = c.checkDataConn()
	if err != nil {
		conn.Close()
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
	_, err := c.conn.Cmd("RNFR %s", from)
	if err != nil {
		return err
	}

	_, _, err = c.conn.ReadCodeLine(StatusRequestFilePending)
	if err != nil {
		return err
	}

	_, err = c.conn.Cmd("RNTO %s", to)
	if err != nil {
		return err
	}

	_, _, err = c.conn.ReadCodeLine(StatusRequestedFileActionOK)
	return err
}

func (c *ServerConn) MakeDir(name string) os.Error {
	// todo
	return nil
}

func (c *ServerConn) RemoveDir(name string) os.Error {
	return nil
}

// Sends a NOOP command. Usualy used to prevent timeouts.
func (c *ServerConn) NoOp() os.Error {
	_, err := c.conn.Cmd("NOOP")
	if err != nil {
		return err
	}
	_, _, err = c.conn.ReadCodeLine(StatusCommandOK)
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
