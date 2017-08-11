package ftp

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	testData = "Just some text"
	testDir  = "mydir"
)

func isTLSServer() bool {
	return os.Getenv("FTP_SERVER") == "vsftpd_implicit_tls"
}

func getConnection() (*ServerConn, error) {
	if isTLSServer() {
		return DialImplicitTLS("localhost:21", &tls.Config{InsecureSkipVerify: true})
	} else {
		return DialTimeout("localhost:21", 5*time.Second)
	}
}

func TestConnPASV(t *testing.T) {
	testConn(t, true)
}

func TestConnEPSV(t *testing.T) {
	testConn(t, false)
}

func TestConnTLS(t *testing.T) {
	if !isTLSServer() {
		t.Skip("skipping test in non TLS server env.")
	}
	testConn(t, false)
}

func testConn(t *testing.T, disableEPSV bool) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	c, err := getConnection()

	if err != nil {
		t.Fatal(err)
	}

	if disableEPSV {
		delete(c.features, "EPSV")
		c.DisableEPSV = true
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

	// Read without deadline
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
		r.Close() // test we can close two times
	}

	// Read with deadline
	r, err = c.Retr("tset")
	if err != nil {
		t.Error(err)
	} else {
		r.SetDeadline(time.Now())
		_, err := ioutil.ReadAll(r)
		if err == nil {
			t.Error("deadline should have caused error")
		} else if !strings.HasSuffix(err.Error(), "i/o timeout") {
			t.Error(err)
		}
		r.Close()
	}

	// Read with offset
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

	fileSize, err := c.FileSize("tset")
	if err != nil {
		t.Error(err)
	}
	if fileSize != 14 {
		t.Errorf("file size %q, expected %q", fileSize, 14)
	}

	data = bytes.NewBufferString("")
	err = c.Stor("tset", data)
	if err != nil {
		t.Error(err)
	}

	fileSize, err = c.FileSize("tset")
	if err != nil {
		t.Error(err)
	}
	if fileSize != 0 {
		t.Errorf("file size %q, expected %q", fileSize, 0)
	}

	_, err = c.FileSize("not-found")
	if err == nil {
		t.Fatal("expected error, got nil")
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
	if isTLSServer() {
		t.Skip("skipping test in TLS mode.")
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
	if testing.Short() || isTLSServer() {
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
	if isTLSServer() {
		t.Skip("skipping test in TLS mode.")
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

	c, err := getConnection()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Quit()

	err = c.Login("zoo2Shia", "fei5Yix9")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDeleteDirRecur(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	c, err := getConnection()
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

	err = c.MakeDir("testDir")
	if err != nil {
		t.Error(err)
	}

	err = c.ChangeDir("testDir")
	if err != nil {
		t.Error(err)
	}

	err = c.MakeDir("anotherDir")
	if err != nil {
		t.Error(err)
	}

	data := bytes.NewBufferString("test text")
	err = c.Stor("fileTest", data)
	if err != nil {
		t.Error(err)
	}

	err = c.ChangeDirToParent()
	if err != nil {
		t.Error(err)
	}
	err = c.RemoveDirRecur("testDir")
	if err != nil {
		t.Error(err)
	}
	dir, err := c.CurrentDir()
	if err != nil {
		t.Error(err)
	} else {
		if dir != "/incoming" {
			t.Error("Wrong dir: " + dir)
		}
	}

	err = c.ChangeDir("testDir")
	if err == nil {
		t.Fatal("expected error, got nil")
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
}

func TestFileDeleteDirRecur(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	c, err := getConnection()
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

	data := bytes.NewBufferString(testData)
	err = c.Stor("testFile", data)
	if err != nil {
		t.Error(err)
	}

	err = c.RemoveDirRecur("testFile")
	if err == nil {
		t.Fatal("expected error got nill")
	}

	dir, err := c.CurrentDir()
	if err != nil {
		t.Error(err)
	} else {
		if dir != "/incoming" {
			t.Error("Wrong dir: " + dir)
		}
	}

	err = c.Delete("testFile")
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
}

func TestMissingFolderDeleteDirRecur(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	c, err := getConnection()
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

	err = c.RemoveDirRecur("test")
	if err == nil {
		t.Fatal("expected error got nill")
	}

	dir, err := c.CurrentDir()
	if err != nil {
		t.Error(err)
	} else {
		if dir != "/incoming" {
			t.Error("Wrong dir: " + dir)
		}
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
}

var globTests = []struct {
	pattern, result string
}{
	{"glob/match.go", "glob/match.go"},
	{"glob/mat?h.go", "glob/match.go"},
	{"glob/ma*ch.go", "glob/match.go"},
	{"**/match.go", "glob/match.go"},
	{"**/*", "glob/match.go"},
}

type MatchTest struct {
	pattern, s string
	match      bool
	err        error
}

var matchTests = []MatchTest{
	{"abc", "abc", true, nil},
	{"*", "abc", true, nil},
	{"*c", "abc", true, nil},
	{"a*", "a", true, nil},
	{"a*", "abc", true, nil},
	{"a*", "ab/c", false, nil},
	{"a*/b", "abc/b", true, nil},
	{"a*/b", "a/c/b", false, nil},
	{"a*b*c*d*e*/f", "axbxcxdxe/f", true, nil},
	{"a*b*c*d*e*/f", "axbxcxdxexxx/f", true, nil},
	{"a*b*c*d*e*/f", "axbxcxdxe/xxx/f", false, nil},
	{"a*b*c*d*e*/f", "axbxcxdxexxx/fff", false, nil},
	{"a*b?c*x", "abxbbxdbxebxczzx", true, nil},
	{"a*b?c*x", "abxbbxdbxebxczzy", false, nil},
	{"ab[c]", "abc", true, nil},
	{"ab[b-d]", "abc", true, nil},
	{"ab[e-g]", "abc", false, nil},
	{"ab[^c]", "abc", false, nil},
	{"ab[^b-d]", "abc", false, nil},
	{"ab[^e-g]", "abc", true, nil},
	{"a\\*b", "a*b", true, nil},
	{"a\\*b", "ab", false, nil},
	{"a?b", "a☺b", true, nil},
	{"a[^a]b", "a☺b", true, nil},
	{"a???b", "a☺b", false, nil},
	{"a[^a][^a][^a]b", "a☺b", false, nil},
	{"[a-ζ]*", "α", true, nil},
	{"*[a-ζ]", "A", false, nil},
	{"a?b", "a/b", false, nil},
	{"a*b", "a/b", false, nil},
	{"[\\]a]", "]", true, nil},
	{"[\\-]", "-", true, nil},
	{"[x\\-]", "x", true, nil},
	{"[x\\-]", "-", true, nil},
	{"[x\\-]", "z", false, nil},
	{"[\\-x]", "x", true, nil},
	{"[\\-x]", "-", true, nil},
	{"[\\-x]", "a", false, nil},
	{"[]a]", "]", false, ErrBadPattern},
	{"[-]", "-", false, ErrBadPattern},
	{"[x-]", "x", false, ErrBadPattern},
	{"[x-]", "-", false, ErrBadPattern},
	{"[x-]", "z", false, ErrBadPattern},
	{"[-x]", "x", false, ErrBadPattern},
	{"[-x]", "-", false, ErrBadPattern},
	{"[-x]", "a", false, ErrBadPattern},
	{"\\", "a", false, ErrBadPattern},
	{"[a-b-c]", "a", false, ErrBadPattern},
	{"[", "a", false, ErrBadPattern},
	{"[^", "a", false, ErrBadPattern},
	{"[^bc", "a", false, ErrBadPattern},
	{"a[", "a", false, nil},
	{"a[", "ab", false, ErrBadPattern},
	{"*x", "xxx", true, nil},
}

func errp(e error) string {
	if e == nil {
		return "<nil>"
	}
	return e.Error()
}

// contains returns true if vector contains the string s.
func contains(vector []string, s string) bool {
	for _, elem := range vector {
		if elem == s {
			return true
		}
	}
	return false
}

type globTest struct {
	pattern string
	matches []string
}

func (test *globTest) buildWant(root string) []string {
	var want []string
	for _, m := range test.matches {
		want = append(want, root+filepath.FromSlash(m))
	}
	sort.Strings(want)
	return want
}

func TestMatch(t *testing.T) {
	for _, tt := range matchTests {
		pattern := tt.pattern
		s := tt.s
		ok, err := Match(pattern, s)
		if ok != tt.match || err != tt.err {
			t.Errorf("Match(%#q, %#q) = %v, %q want %v, %q", pattern, s, ok, errp(err), tt.match, errp(tt.err))
		}
	}
}

func TestGlob(t *testing.T) {

	c, err := getConnection()
	if err != nil {
		t.Fatal(err)
	}
	if !c.mlstSupported {
		t.Skip("skipping test, server not supporting MLST.")
	}
	err = c.Login("anonymous", "anonymous")
	if err != nil {
		t.Fatal(err)
	}

	err = c.ChangeDir("incoming")
	if err != nil {
		t.Error(err)
	}

	err = c.MakeDir("glob")
	if err != nil {
		t.Error(err)
	}

	data := bytes.NewBufferString("")
	err = c.Stor("glob/match.go", data)
	if err != nil {
		t.Error(err)
	}

	for _, tt := range globTests {
		pattern := tt.pattern
		result := tt.result
		matches, err := c.Glob(pattern)
		if err != nil {
			t.Errorf("Glob error for %q: %s", pattern, err)
			continue
		}
		if !contains(matches, result) {
			t.Errorf("Glob(%#q) = %#v want %v", pattern, matches, result)
		}
	}
	for _, pattern := range []string{"no_match", "../*/no_match"} {
		matches, err := c.Glob(pattern)
		if err != nil {
			t.Errorf("Glob error for %q: %s", pattern, err)
			continue
		}
		if len(matches) != 0 {
			t.Errorf("Glob(%#q) = %#v want []", pattern, matches)
		}
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
}

func TestGlobError(t *testing.T) {
	c, err := getConnection()
	if err != nil {
		t.Fatal(err)
	}
	if !c.mlstSupported {
		t.Skip("skipping test, server not supporting MLST.")
	}

	err = c.Login("anonymous", "anonymous")
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.Glob("[7]")
	if err != nil {
		t.Error("expected error for bad pattern; got none")
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
}
