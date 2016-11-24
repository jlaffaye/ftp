package ftp

import (
	"bytes"
	"io/ioutil"
	"net/textproto"
	"sync"
	"testing"
	"time"
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

func testConn(t *testing.T, passive bool) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	c, err := DialTimeout("localhost:21", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if passive {
		delete(c.features, "EPSV")
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

	r, err = c.RetrFrom("tset", 5)
	if err != nil {
		t.Error(err)
	} else {
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			t.Error(err)
		}
		expected := testData[5:]
		if string(buf) != expected {
			t.Errorf("read %q, expected %q", buf, expected)
		}
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

	entries, err := c.NameList("/")
	if err != nil {
		t.Error(err)
	}
	if len(entries) != 1 || entries[0] != "/incoming" {
		t.Errorf("Unexpected entries: %v", entries)
	}

	err = c.RemoveDir(testDir)
	if err != nil {
		t.Error(err)
	}

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

	c.Quit()

	err = c.NoOp()
	if err == nil {
		t.Error("Expected error")
	}
}

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

// TestConnect tests the legacy Connect function
func TestConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	c, err := Connect("localhost:21")
	if err != nil {
		t.Fatal(err)
	}

	c.Quit()
}

func TestTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	c, err := DialTimeout("localhost:2121", 1*time.Second)
	if err == nil {
		t.Fatal("expected timeout, got nil error")
		c.Quit()
	}
}

func TestWrongLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	c, err := DialTimeout("localhost:21", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Quit()

	err = c.Login("zoo2Shia", "fei5Yix9")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConcurrentAccess(t *testing.T) {
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

	err = c.ChangeDir("incoming")
	if err != nil {
		t.Error(err)
	}

	wg := sync.WaitGroup{}

	files := []string{"test1", "test2"}
	for _, f := range files {
		wg.Add(1)

		go func(fn string) {
			data := bytes.NewBufferString(testData)
			err := c.Stor(fn, data)
			if err != nil {
				t.Error(err)
			}

			_, err = c.List(".")
			if err != nil {
				t.Error(err)
			}

			err = c.Rename(fn, fn+"tset")
			if err != nil {
				t.Error(err)
			}

			r, err := c.Retr(fn + "tset")
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

			r, err = c.RetrFrom(fn+"tset", 5)
			if err != nil {
				t.Error(err)
			} else {
				buf, err := ioutil.ReadAll(r)
				if err != nil {
					t.Error(err)
				}
				expected := testData[5:]
				if string(buf) != expected {
					t.Errorf("read %q, expected %q", buf, expected)
				}
				r.Close()
			}

			err = c.Delete(fn + "tset")
			if err != nil {
				t.Error(err)
			}

			wg.Done()
		}(f)
	}

	wg.Wait()

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

	c.Quit()

	err = c.NoOp()
	if err == nil {
		t.Error("Expected error")
	}
}
