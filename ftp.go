package ftp

import (
	"bufio"
	"net"
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
	conn net.Conn
	bio *bufio.Reader
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

// Check if the last status code is equal to the given code
// If it is the case, err is nil
// Returns the status line for further processing
func (c *ServerConn) checkStatus(expected int) (line string, err os.Error) {
	line, err = c.bio.ReadString('\n')
	if err != nil {
		return
	}
	code, err := strconv.Atoi(line[:3]) // A status is 3 digits
	if err != nil {
		return
	}
	if code != expected {
		err = os.NewError(fmt.Sprintf("%d %s", code, statusText[code]))
		return
	}
	return
}

// Like send() but with formating.
func (c *ServerConn) sendf(str string, a ...interface{}) (os.Error) {
	return c.send([]byte(fmt.Sprintf(str, a...)))
}

// Send a raw command on the connection.
func (c *ServerConn) send(data []byte) (os.Error) {
	_, err := c.conn.Write(data)
	return err
}

// Connect to a ftp server and returns a ServerConn handler.
func Connect(host, user, password string) (*ServerConn, os.Error) {
	conn, err := net.Dial("tcp", host)
	if err != nil {
		return nil, err
	}

	c := &ServerConn{conn, bufio.NewReader(conn)}

	_, err = c.checkStatus(StatusReady)
	if err != nil {
		c.Close()
		return nil, err
	}

	c.sendf("USER %v\r\n", user)
	_, err = c.checkStatus(StatusUserOK)
	if err != nil {
		c.Close()
		return nil, err
	}

	c.sendf("PASS %v\r\n", password)
	_, err = c.checkStatus(StatusLoggedIn)
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
	c.send([]byte("EPSV\r\n"))
	line, err := c.checkStatus(StatusExtendedPassiveMode)
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
	a := strings.Split(c.conn.RemoteAddr().String(), ":", 2)
	addr := fmt.Sprintf("%v:%v", a[0], port)

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

	c.send([]byte("LIST\r\n"))
	_, err = c.checkStatus(StatusAboutToSend)
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
	c.sendf("CWD %s\r\n", path);
	_, err = c.checkStatus(StatusRequestedFileActionOK)
	return
}

func (c *ServerConn) Get(path string) (r *Response, err os.Error) {
	r, err = c.openDataConnection()
	if err != nil {
		return
	}

	c.sendf("RETR %s\r\n", path)
	_, err = c.checkStatus(StatusAboutToSend)
	return
}

func (c *ServerConn) Close() {
	c.send([]byte("QUIT\r\n"))
	c.conn.Close()
}

func (r *Response) Read(buf []byte) (int, os.Error) {
	n, err := r.conn.Read(buf)
	if err == os.EOF {
		_, err2 := r.c.checkStatus(StatusClosingDataConnection)
		if err2 != nil {
			err = err2
		}
	}
	return n, err
}

func (r *Response) Close() os.Error {
	return r.conn.Close()
}
