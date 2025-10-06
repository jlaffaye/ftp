package ftp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	assert := assert.New(t)
	mock, c := openConn(t, "127.0.0.1", DialWithTimeout(5*time.Second), DialWithDisabledEPSV(disableEPSV))

	err := c.Login("anonymous", "anonymous")
	assert.NoError(err)

	err = c.NoOp()
	assert.NoError(err)

	err = c.ChangeDir("incoming")
	assert.NoError(err)

	dir, err := c.CurrentDir()
	if assert.NoError(err) {
		assert.Equal("/incoming", dir)
	}

	data := bytes.NewBufferString(testData)
	err = c.Stor("test", data)
	assert.NoError(err)

	err = c.Chmod("test", 0o755)
	assert.NoError(err)

	_, err = c.List(".")
	assert.NoError(err)

	err = c.Rename("test", "tset")
	assert.NoError(err)

	// Read without deadline
	r, err := c.Retr("tset")
	if assert.NoError(err) {
		buf, err := io.ReadAll(r)
		if assert.NoError(err) {
			assert.Equal(testData, string(buf))
		}

		r.Close()
		r.Close() // test we can close two times
	}

	// Read with deadline
	r, err = c.Retr("tset")
	if assert.NoError(err) {
		if err := r.SetDeadline(time.Now()); err != nil {
			t.Fatal(err)
		}
		_, err = io.ReadAll(r)
		assert.ErrorContains(err, "i/o timeout")
		r.Close()
	}

	// Read with offset
	r, err = c.RetrFrom("tset", 5)
	if assert.NoError(err) {
		buf, err := io.ReadAll(r)
		if assert.NoError(err) {
			expected := testData[5:]
			assert.Equal(expected, string(buf))
		}

		r.Close()
	}

	data2 := bytes.NewBufferString(testData)
	err = c.Append("tset", data2)
	assert.NoError(err)

	// Read without deadline, after append
	r, err = c.Retr("tset")
	if assert.NoError(err) {
		buf, err := io.ReadAll(r)
		if assert.NoError(err) {
			assert.Equal(testData+testData, string(buf))
		}

		r.Close()
	}

	fileSize, err := c.FileSize("magic-file")
	assert.NoError(err)
	assert.Equal(int64(42), fileSize)

	_, err = c.FileSize("not-found")
	assert.Error(err)

	entry, err := c.GetEntry("magic-file")
	if err != nil {
		t.Error(err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Size != 42 {
		t.Errorf("entry size %q, expected %q", entry.Size, 42)
	}
	if entry.Type != EntryTypeFile {
		t.Errorf("entry type %q, expected %q", entry.Type, EntryTypeFile)
	}
	if entry.Name != "magic-file" {
		t.Errorf("entry name %q, expected %q", entry.Name, "magic-file")
	}
	assert.Equal(0o644, entry.FileMode.Perm())

	entry, err = c.GetEntry("multiline-dir")
	if err != nil {
		t.Error(err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Size != 0 {
		t.Errorf("entry size %q, expected %q", entry.Size, 0)
	}
	if entry.Type != EntryTypeFolder {
		t.Errorf("entry type %q, expected %q", entry.Type, EntryTypeFolder)
	}
	if entry.Name != "multiline-dir" {
		t.Errorf("entry name %q, expected %q", entry.Name, "multiline-dir")
	}
	assert.Equal(0o755, entry.FileMode.Perm())
	err = c.Chmod("multiline-dir", 0o744)
	assert.NoError(err)

	err = c.Delete("tset")
	assert.NoError(err)

	err = c.MakeDir(testDir)
	assert.NoError(err)

	err = c.ChangeDir(testDir)
	assert.NoError(err)

	err = c.ChangeDirToParent()
	assert.NoError(err)

	entries, err := c.NameList("/")
	assert.NoError(err)
	assert.Equal([]string{"/incoming"}, entries)

	err = c.RemoveDir(testDir)
	assert.NoError(err)

	err = c.Logout()
	assert.NoError(err)

	if err = c.Quit(); err != nil {
		t.Fatal(err)
	}

	// Wait for the connection to close
	mock.Wait()

	err = c.NoOp()
	assert.Error(err, "should error on closed conn")
}

// TestConnect tests the legacy Connect function
func TestConnect(t *testing.T) {
	mock, err := newFtpMock(t, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	c, err := Connect(mock.Addr())
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Quit(); err != nil {
		t.Fatal(err)
	}
	mock.Wait()
}

func TestTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	if c, err := DialTimeout("localhost:2121", 1*time.Second); err == nil {
		_ = c.Quit()
		t.Fatal("expected timeout, got nil error")
	}
}

func TestWrongLogin(t *testing.T) {
	mock, err := newFtpMock(t, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	c, err := DialTimeout(mock.Addr(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := c.Quit(); err != nil {
			t.Errorf("can not quit: %s", err)
		}
	}()

	err = c.Login("zoo2Shia", "fei5Yix9")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDeleteDirRecur(t *testing.T) {
	mock, c := openConn(t, "127.0.0.1")

	err := c.RemoveDirRecur("testDir")
	if err != nil {
		t.Error(err)
	}

	if err := c.Quit(); err != nil {
		t.Fatal(err)
	}

	// Wait for the connection to close
	mock.Wait()
}

// func TestFileDeleteDirRecur(t *testing.T) {
// 	mock, c := openConn(t, "127.0.0.1")

// 	err := c.RemoveDirRecur("testFile")
// 	if err == nil {
// 		t.Fatal("expected error got nil")
// 	}

// 	if err := c.Quit(); err != nil {
// 		t.Fatal(err)
// 	}

// 	// Wait for the connection to close
// 	mock.Wait()
// }

func TestMissingFolderDeleteDirRecur(t *testing.T) {
	mock, c := openConn(t, "127.0.0.1")

	err := c.RemoveDirRecur("missing-dir")
	if err == nil {
		t.Fatal("expected error got nil")
	}

	if err := c.Quit(); err != nil {
		t.Fatal(err)
	}

	// Wait for the connection to close
	mock.Wait()
}

func TestListCurrentDir(t *testing.T) {
	mock, c := openConnExt(t, "127.0.0.1", "no-time", DialWithDisabledMLSD(true))

	_, err := c.List("")
	assert.NoError(t, err)
	assert.Equal(t, "LIST", mock.lastFull, "LIST must not have a trailing whitespace")

	_, err = c.NameList("")
	assert.NoError(t, err)
	assert.Equal(t, "NLST", mock.lastFull, "NLST must not have a trailing whitespace")

	err = c.Quit()
	assert.NoError(t, err)

	mock.Wait()
}

func TestListCurrentDirWithForceListHidden(t *testing.T) {
	mock, c := openConnExt(t, "127.0.0.1", "no-time", DialWithDisabledMLSD(true), DialWithForceListHidden(true))

	assert.True(t, c.options.forceListHidden)
	_, err := c.List("")
	assert.NoError(t, err)
	assert.Equal(t, "LIST -a", mock.lastFull, "LIST -a must not have a trailing whitespace")

	err = c.Quit()
	assert.NoError(t, err)

	mock.Wait()
}

func TestTimeUnsupported(t *testing.T) {
	mock, c := openConnExt(t, "127.0.0.1", "no-time")

	assert.False(t, c.mdtmSupported, "MDTM must NOT be supported")
	assert.False(t, c.mfmtSupported, "MFMT must NOT be supported")

	assert.False(t, c.IsGetTimeSupported(), "GetTime must NOT be supported")
	assert.False(t, c.IsSetTimeSupported(), "SetTime must NOT be supported")

	_, err := c.GetTime("file1")
	assert.NotNil(t, err)

	err = c.SetTime("file1", time.Now())
	assert.NotNil(t, err)

	assert.NoError(t, c.Quit())
	mock.Wait()
}

func TestTimeStandard(t *testing.T) {
	mock, c := openConnExt(t, "127.0.0.1", "std-time")

	assert.True(t, c.mdtmSupported, "MDTM must be supported")
	assert.True(t, c.mfmtSupported, "MFMT must be supported")

	assert.True(t, c.IsGetTimeSupported(), "GetTime must be supported")
	assert.True(t, c.IsSetTimeSupported(), "SetTime must be supported")

	tm, err := c.GetTime("file1")
	assert.NoError(t, err)
	assert.False(t, tm.IsZero(), "GetTime must return valid time")

	err = c.SetTime("file1", time.Now())
	assert.NoError(t, err)

	assert.NoError(t, c.Quit())
	mock.Wait()
}

func TestTimeVsftpdPartial(t *testing.T) {
	mock, c := openConnExt(t, "127.0.0.1", "vsftpd")

	assert.True(t, c.mdtmSupported, "MDTM must be supported")
	assert.False(t, c.mfmtSupported, "MFMT must NOT be supported")

	assert.True(t, c.IsGetTimeSupported(), "GetTime must be supported")
	assert.False(t, c.IsSetTimeSupported(), "SetTime must NOT be supported")

	tm, err := c.GetTime("file1")
	assert.NoError(t, err)
	assert.False(t, tm.IsZero(), "GetTime must return valid time")

	err = c.SetTime("file1", time.Now())
	assert.NotNil(t, err)

	assert.NoError(t, c.Quit())
	mock.Wait()
}

func TestTimeVsftpdFull(t *testing.T) {
	mock, c := openConnExt(t, "127.0.0.1", "vsftpd", DialWithWritingMDTM(true))

	assert.True(t, c.mdtmSupported, "MDTM must be supported")
	assert.False(t, c.mfmtSupported, "MFMT must NOT be supported")

	assert.True(t, c.IsGetTimeSupported(), "GetTime must be supported")
	assert.True(t, c.IsSetTimeSupported(), "SetTime must be supported")

	tm, err := c.GetTime("file1")
	assert.NoError(t, err)
	assert.False(t, tm.IsZero(), "GetTime must return valid time")

	err = c.SetTime("file1", time.Now())
	assert.NoError(t, err)

	assert.NoError(t, c.Quit())
	mock.Wait()
}

func TestDialWithDialFunc(t *testing.T) {
	dialErr := fmt.Errorf("this is proof that dial function was called")

	f := func(network, address string) (net.Conn, error) {
		return nil, dialErr
	}

	_, err := Dial("bogus-address", DialWithDialFunc(f))
	assert.Equal(t, dialErr, err)
}

func TestDialWithDialer(t *testing.T) {
	dialerCalled := false
	dialer := net.Dialer{
		Control: func(network, address string, c syscall.RawConn) error {
			dialerCalled = true
			return nil
		},
	}

	mock, err := newFtpMock(t, "127.0.0.1")
	assert.NoError(t, err)

	c, err := Dial(mock.Addr(), DialWithDialer(dialer))
	assert.NoError(t, err)
	assert.NoError(t, c.Quit())

	assert.Equal(t, true, dialerCalled)
}
