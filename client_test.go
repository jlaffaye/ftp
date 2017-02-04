package ftp

import (
	"bytes"
	"io/ioutil"
	"net/textproto"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	testData = "Just some text"
	testDir  = "mydir"
)

func TestConnPASV(t *testing.T) {
	testConn(t, true)
}

func TestConnEPSV(t *testing.T) {
	testConn(t, false)
}

func testConn(t *testing.T, disableEPSV bool) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	require := require.New(t)

	c, err := DialTimeout("localhost:21", 5*time.Second)
	require.NoError(err)

	if disableEPSV {
		delete(c.features, "EPSV")
		c.disableEPSV = true
	}

	err = c.Login("anonymous", "anonymous")
	require.NoError(err)

	err = c.NoOp()
	require.NoError(err)

	err = c.ChangeDir("incoming")
	require.NoError(err)

	data := bytes.NewBufferString(testData)
	err = c.Stor("test", data)
	require.NoError(err)

	_, err = c.List(".")
	require.NoError(err)

	err = c.Rename("test", "tset")
	require.NoError(err)

	r, err := c.Retr("tset")
	require.NoError(err)
	buf, err := ioutil.ReadAll(r)
	require.NoError(err)
	require.Equal(testData, string(buf))
	err = r.Close()
	require.NoError(err)

	r, err = c.RetrFrom("tset", 5)
	require.NoError(err)
	buf, err = ioutil.ReadAll(r)
	require.NoError(err)
	require.Equal(testData[5:], string(buf))
	r.Close()

	err = c.Delete("tset")
	require.NoError(err)

	err = c.MakeDir(testDir)
	require.NoError(err)

	err = c.ChangeDir(testDir)
	require.NoError(err)

	dir, err := c.CurrentDir()
	require.NoError(err)
	require.Equal("/incoming/"+testDir, dir)

	err = c.ChangeDirToParent()
	require.NoError(err)

	entries, err := c.NameList("/")
	require.NoError(err)
	require.EqualValues([]string{"/incoming"}, entries)

	err = c.RemoveDir(testDir)
	require.NoError(err)

	err = c.Logout()
	if err != nil {
		if protoErr := err.(*textproto.Error); protoErr != nil {
			if protoErr.Code != StatusNotImplemented {
				t.Error(err)
			}
		} else {
			t.Error(err)
		}
	}

	err = c.Quit()
	require.NoError(err)

	err = c.NoOp()
	require.Error(err)
	require.Regexp("write tcp .* use of closed network connection", err.Error())
}

func TestConnIPv6(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	require := require.New(t)

	c, err := DialTimeout("[::1]:21", 5*time.Second)
	require.NoError(err)

	err = c.Login("anonymous", "anonymous")
	require.NoError(err)

	_, err = c.List(".")
	require.NoError(err)

	err = c.Quit()
	require.NoError(err)
}

// TestConnect tests the legacy Connect function
func TestConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	c, err := Connect("localhost:21")
	require.NoError(t, err)

	c.Quit()
}

func TestDialTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	require := require.New(t)

	_, err := DialTimeout("localhost:2121", 1*time.Second)
	require.Error(err)
	require.Regexp("dial tcp .* connection refused", err.Error())
}

func TestWrongLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	require := require.New(t)

	c, err := DialTimeout("localhost:21", 5*time.Second)
	require.NoError(err)
	defer c.Quit()

	err = c.Login("zoo2Shia", "fei5Yix9")
	require.Error(err)
	require.Regexp("(Login incorrect|anonymous only)", err.Error())
}
