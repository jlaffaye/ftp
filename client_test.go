package ftp

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"
)

const (
	testData = "Just some text"
	testDir  = "mydir"
)

func TestConn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	c, err := DialTimeout("localhost:21", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	err = c.Login("anonymous", "anonymous")
	if err != nil {
		t.Fatal(err)
	}

	err = c.NoOp()
	if err != nil {
		t.Error(err)
	}

	err = c.ChangeDir("incoming")
	if err != nil {
		t.Error(err)
	}

	data := bytes.NewBufferString(testData)
	err = c.Stor("test", data)
	if err != nil {
		t.Error(err)
	}

	_, err = c.List(".")
	if err != nil {
		t.Error(err)
	}

	err = c.Rename("test", "tset")
	if err != nil {
		t.Error(err)
	}

	r, err := c.Retr("tset")
	if err != nil {
		t.Error(err)
	} else {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			t.Error(err)
		}
		if string(buf) != testData {
			t.Errorf("'%s'", buf)
		}
		r.Close()
	}

	r, err = c.Retr("tset")
	if err != nil {
		t.Error(err)
	} else {
		r.Close()
	}

	err = c.Delete("tset")
	if err != nil {
		t.Error(err)
	}

	err = c.MakeDir(testDir)
	if err != nil {
		t.Error(err)
	}

	err = c.ChangeDir(testDir)
	if err != nil {
		t.Error(err)
	}

	dir, err := c.CurrentDir()
	if err != nil {
		t.Error(err)
	} else {
		if dir != "/incoming/"+testDir {
			t.Error("Wrong dir: " + dir)
		}
	}

	err = c.ChangeDirToParent()
	if err != nil {
		t.Error(err)
	}

	err = c.RemoveDir(testDir)
	if err != nil {
		t.Error(err)
	}

	err = c.Logout()
	// REIN is not supported by vsftpd
	if err != nil && err.Error() != "502 REIN not implemented." {
		t.Error(err)
	}

	c.Quit()

	err = c.NoOp()
	if err == nil {
		t.Error("Expected error")
	}
}

// ftp.mozilla.org uses multiline 220 response
func TestMultiline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	c, err := DialTimeout("ftp.mozilla.org:21", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	err = c.Login("anonymous", "anonymous")
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.List(".")
	if err != nil {
		t.Error(err)
	}

	c.Quit()
}

// antioche.antioche.eu.org with IPv6
func TestConnIPv6(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	c, err := DialTimeout("[::1]:21", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	err = c.Login("anonymous", "anonymous")
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.List(".")
	if err != nil {
		t.Error(err)
	}

	c.Quit()
}
