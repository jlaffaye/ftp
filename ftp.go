package ftp

import (
	"bufio"
	"net"
	"net/textproto"
	"os"
	"fmt"
	"strconv"
	"strings"
)

const (
	EntryTypeFile = iota
	EntryTypeFolder
	EntryTypeLink
)

type ServerConn struct {
	conn *textproto.Conn
	host string
}

type Response struct {
	conn net.Conn
	c *ServerConn
}

type Entry struct {
	Name string
	EntryType int
	Size uint64
}

// Connect to a ftp server and returns a ServerConn handler.
func Connect(host, user, password string) (*ServerConn, os.Error) {
	conn, err := textproto.Dial("tcp", host)
	if err != nil {
		return nil, err
	}

	a := strings.Split(host, ":", 2)
	c := &ServerConn{conn, a[0]}

	_, _, err = c.conn.ReadCodeLine(StatusReady)
	if err != nil {
		c.Close()
		return nil, err
	}

	c.conn.Cmd("USER %s", user)
	_, _, err = c.conn.ReadCodeLine(StatusUserOK)
	if err != nil {
		c.Close()
		return nil, err
	}

	c.conn.Cmd("PASS %s", password)
	_, _, err = c.conn.ReadCodeLine(StatusLoggedIn)
	if err != nil {
		c.Close()
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
func (c *ServerConn) openDataConnection() (r *Response, err os.Error) {
	port, err := c.epsv()
	if err != nil {
		return
	}

	// Build the new net address string
	addr := fmt.Sprintf("%s:%d", c.host, port)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return
	}

	r = &Response{conn, c}
	return
}

func parseListLine(line string) (*Entry, os.Error) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return nil, os.NewError("Unsupported LIST line")
	}

	e := &Entry{}
	switch fields[0][0] {
		case '-':
			e.EntryType = EntryTypeFile
		case 'd':
			e.EntryType = EntryTypeFolder
		case 'l':
			e.EntryType = EntryTypeLink
		default:
			return nil, os.NewError("Unknown entry type")
	}

	e.Name = strings.Join(fields[8:], " ")
	return e, nil
}

func (c *ServerConn) List() (entries []*Entry, err os.Error) {
	r, err := c.openDataConnection()
	if err != nil {
		return
	}
	defer r.Close()

	c.conn.Cmd("LIST")
	_, _, err = c.conn.ReadCodeLine(StatusAboutToSend)
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
	c.conn.Cmd("CWD %s", path);
	_, _, err = c.conn.ReadCodeLine(StatusRequestedFileActionOK)
	return
}

func (c *ServerConn) Get(path string) (r *Response, err os.Error) {
	r, err = c.openDataConnection()
	if err != nil {
		return
	}

	c.conn.Cmd("RETR %s", path)
	_, _, err = c.conn.ReadCodeLine(StatusAboutToSend)
	return
}

func (c *ServerConn) Close() {
	c.conn.Cmd("QUIT")
	c.conn.Close()
}

func (r *Response) Read(buf []byte) (int, os.Error) {
	n, err := r.conn.Read(buf)
	if err == os.EOF {
		_, _, err2 := r.c.conn.ReadCodeLine(StatusClosingDataConnection)
		if err2 != nil {
			err = err2
		}
	}
	return n, err
}

func (r *Response) Close() os.Error {
	return r.conn.Close()
}
