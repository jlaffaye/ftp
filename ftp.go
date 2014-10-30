// Package ftp implements a FTP client as described in RFC 959.
package ftp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// EntryType describes the different types of an Entry.
type EntryType int

const (
	EntryTypeFile EntryType = iota
	EntryTypeFolder
	EntryTypeLink
)

// ServerConn represents the connection to a remote FTP server.
type ServerConn struct {
	conn     *textproto.Conn
	host     string
	features map[string]string
}

// Entry describes a file and is returned by List().
type Entry struct {
	Name string
	Type EntryType
	Size uint64
	Time time.Time
}

// response represent a data-connection
type response struct {
	conn net.Conn
	c    *ServerConn
}

// Connect initializes the connection to the specified ftp server address.
//
// It is generally followed by a call to Login() as most FTP commands require
// an authenticated user.
func Connect(addr string) (*ServerConn, error) {
	conn, err := textproto.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	a := strings.SplitN(addr, ":", 2)
	c := &ServerConn{
		conn:     conn,
		host:     a[0],
		features: make(map[string]string),
	}

	_, _, err = c.conn.ReadResponse(StatusReady)
	if err != nil {
		c.Quit()
		return nil, err
	}

	err = c.feat()
	if err != nil {
		c.Quit()
		return nil, err
	}

	return c, nil
}

// Login authenticates the client with specified user and password.
//
// "anonymous"/"anonymous" is a common user/password scheme for FTP servers
// that allows anonymous read-only accounts.
func (c *ServerConn) Login(user, password string) error {
	code, message, err := c.cmd(-1, "USER %s", user)
	if err != nil {
		return err
	}

	switch code {
	case StatusLoggedIn:
	case StatusUserOK:
		_, _, err = c.cmd(StatusLoggedIn, "PASS %s", password)
		if err != nil {
			return err
		}
	default:
		return errors.New(message)
	}

	// Switch to binary mode
	_, _, err = c.cmd(StatusCommandOK, "TYPE I")
	if err != nil {
		return err
	}

	return nil
}

// feat issues a FEAT FTP command to list the additional commands supported by
// the remote FTP server.
// FEAT is described in RFC 2389
func (c *ServerConn) feat() error {
	code, message, err := c.cmd(-1, "FEAT")
	if err != nil {
		return err
	}

	if code != StatusSystem {
		// The server does not support the FEAT command. This is not an
		// error: we consider that there is no additional feature.
		return nil
	}

	lines := strings.Split(message, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, " ") {
			continue
		}

		line = strings.TrimSpace(line)
		featureElements := strings.SplitN(line, " ", 2)

		command := featureElements[0]

		var commandDesc string
		if len(featureElements) == 2 {
			commandDesc = featureElements[1]
		}

		c.features[command] = commandDesc
	}

	return nil
}

// epsv issues an "EPSV" command to get a port number for a data connection.
func (c *ServerConn) epsv() (port int, err error) {
	_, line, err := c.cmd(StatusExtendedPassiveMode, "EPSV")
	if err != nil {
		return
	}

	start := strings.Index(line, "|||")
	end := strings.LastIndex(line, "|")
	if start == -1 || end == -1 {
		err = errors.New("Invalid EPSV response format")
		return
	}
	port, err = strconv.Atoi(line[start+3 : end])
	return
}

// pasv issues a "PASV" command to get a port number for a data connection.
func (c *ServerConn) pasv() (port int, err error) {
	_, line, err := c.cmd(StatusPassiveMode, "PASV")
	if err != nil {
		return
	}

	// PASV response format : 227 Entering Passive Mode (h1,h2,h3,h4,p1,p2).
	start := strings.Index(line, "(")
	end := strings.LastIndex(line, ")")
	if start == -1 || end == -1 {
		err = errors.New("Invalid EPSV response format")
		return
	}

	// We have to split the response string
	pasvData := strings.Split(line[start+1:end], ",")
	// Let's compute the port number
	portPart1, err1 := strconv.Atoi(pasvData[4])
	if err1 != nil {
		err = err1
		return
	}

	portPart2, err2 := strconv.Atoi(pasvData[5])
	if err2 != nil {
		err = err2
		return
	}

	// Recompose port
	port = portPart1*256 + portPart2
	return
}

// openDataConn creates a new FTP data connection.
func (c *ServerConn) openDataConn() (net.Conn, error) {
	var port int
	var err error

	//  If features contains nat6 or EPSV => EPSV
	//  else -> PASV
	_, nat6Supported := c.features["nat6"]
	_, epsvSupported := c.features["EPSV"]
	if nat6Supported || epsvSupported {
		port, err = c.epsv()
		if err != nil {
			return nil, err
		}
	} else {
		port, err = c.pasv()
		if err != nil {
			return nil, err
		}
	}

	// Build the new net address string
	addr := fmt.Sprintf("%s:%d", c.host, port)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// cmd is a helper function to execute a command and check for the expected FTP
// return code
func (c *ServerConn) cmd(expected int, format string, args ...interface{}) (int, string, error) {
	_, err := c.conn.Cmd(format, args...)
	if err != nil {
		return 0, "", err
	}

	code, line, err := c.conn.ReadResponse(expected)
	return code, line, err
}

// cmdDataConn executes a command which require a FTP data connection.
func (c *ServerConn) cmdDataConn(format string, args ...interface{}) (net.Conn, error) {
	conn, err := c.openDataConn()
	if err != nil {
		return nil, err
	}

	_, err = c.conn.Cmd(format, args...)
	if err != nil {
		conn.Close()
		return nil, err
	}

	code, msg, err := c.conn.ReadResponse(-1)
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

// parseListLine parses the various non-standard format returned by the LIST
// FTP command.
func parseListLine(line string) (*Entry, error) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return nil, errors.New("Unsupported LIST line")
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
		return nil, errors.New("Unknown entry type")
	}

	if e.Type == EntryTypeFile {
		size, err := strconv.ParseUint(fields[4], 10, 0)
		if err != nil {
			return nil, err
		}
		e.Size = size
	}
	var timeStr string
	if strings.Contains(fields[7], ":") { // this year
		thisYear, _, _ := time.Now().Date()
		timeStr = fields[6] + " " + fields[5] + " " + strconv.Itoa(thisYear)[2:4] + " " + fields[7] + " GMT"
	} else { // not this year
		timeStr = fields[6] + " " + fields[5] + " " + fields[7][2:4] + " " + "00:00" + " GMT"
	}
	t, err := time.Parse("_2 Jan 06 15:04 MST", timeStr)
	if err != nil {
		return nil, err
	}
	e.Time = t

	e.Name = strings.Join(fields[8:], " ")
	return e, nil
}

// NameList issues an NLST FTP command.
func (c *ServerConn) NameList(path string) (entries []string, err error) {
	conn, err := c.cmdDataConn("NLST %s", path)
	if err != nil {
		return
	}

	r := &response{conn, c}
	defer r.Close()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		entries = append(entries, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		return entries, err
	}
	return
}

// List issues a LIST FTP command.
func (c *ServerConn) List(path string) (entries []*Entry, err error) {
	conn, err := c.cmdDataConn("LIST %s", path)
	if err != nil {
		return
	}

	r := &response{conn, c}
	defer r.Close()

	bio := bufio.NewReader(r)
	for {
		line, e := bio.ReadString('\n')
		if e == io.EOF {
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

// ChangeDir issues a CWD FTP command, which changes the current directory to
// the specified path.
func (c *ServerConn) ChangeDir(path string) error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "CWD %s", path)
	return err
}

// ChangeDirToParent issues a CDUP FTP command, which changes the current
// directory to the parent directory.  This is similar to a call to ChangeDir
// with a path set to "..".
func (c *ServerConn) ChangeDirToParent() error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "CDUP")
	return err
}

// CurrentDir issues a PWD FTP command, which Returns the path of the current
// directory.
func (c *ServerConn) CurrentDir() (string, error) {
	_, msg, err := c.cmd(StatusPathCreated, "PWD")
	if err != nil {
		return "", err
	}

	start := strings.Index(msg, "\"")
	end := strings.LastIndex(msg, "\"")

	if start == -1 || end == -1 {
		return "", errors.New("Unsuported PWD response format")
	}

	return msg[start+1 : end], nil
}

// Retr issues a RETR FTP command to fetch the specified file from the remote
// FTP server.
//
// The returned ReadCloser must be closed to cleanup the FTP data connection.
func (c *ServerConn) Retr(path string) (io.ReadCloser, error) {
	conn, err := c.cmdDataConn("RETR %s", path)
	if err != nil {
		return nil, err
	}

	r := &response{conn, c}
	return r, nil
}

// Stor issues a STOR FTP command to store a file to the remote FTP server.
// Stor creates the specified file with the content of the io.Reader.
//
// Hint: io.Pipe() can be used if an io.Writer is required.
func (c *ServerConn) Stor(path string, r io.Reader) error {
	conn, err := c.cmdDataConn("STOR %s", path)
	if err != nil {
		return err
	}

	_, err = io.Copy(conn, r)
	conn.Close()
	if err != nil {
		return err
	}

	_, _, err = c.conn.ReadResponse(StatusClosingDataConnection)
	return err
}

// Rename renames a file on the remote FTP server.
func (c *ServerConn) Rename(from, to string) error {
	_, _, err := c.cmd(StatusRequestFilePending, "RNFR %s", from)
	if err != nil {
		return err
	}

	_, _, err = c.cmd(StatusRequestedFileActionOK, "RNTO %s", to)
	return err
}

// Delete issues a DELE FTP command to delete the specified file from the
// remote FTP server.
func (c *ServerConn) Delete(path string) error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "DELE %s", path)
	return err
}

// MakeDir issues a MKD FTP command to create the specified directory on the
// remote FTP server.
func (c *ServerConn) MakeDir(path string) error {
	_, _, err := c.cmd(StatusPathCreated, "MKD %s", path)
	return err
}

// RemoveDir issues a RMD FTP command to remove the specified directory from
// the remote FTP server.
func (c *ServerConn) RemoveDir(path string) error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "RMD %s", path)
	return err
}

// NoOp issues a NOOP FTP command.
// NOOP has no effects and is usually used to prevent the remote FTP server to
// close the otherwise idle connection.
func (c *ServerConn) NoOp() error {
	_, _, err := c.cmd(StatusCommandOK, "NOOP")
	return err
}

// Logout issues a REIN FTP command to logout the current user.
func (c *ServerConn) Logout() error {
	_, _, err := c.cmd(StatusLoggedIn, "REIN")
	return err
}

// Quit issues a QUIT FTP command to properly close the connection from the
// remote FTP server.
func (c *ServerConn) Quit() error {
	c.conn.Cmd("QUIT")
	return c.conn.Close()
}

// Read implements the io.Reader interface on a FTP data connection.
func (r *response) Read(buf []byte) (int, error) {
	n, err := r.conn.Read(buf)
	return n, err
}

// Close implements the io.Closer interface on a FTP data connection.
func (r *response) Close() error {
	err := r.conn.Close()
	_, _, err2 := r.c.conn.ReadResponse(StatusClosingDataConnection)
	if err2 != nil {
		err = err2
	}
	return err
}
