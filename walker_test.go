package ftp

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFieldsReturnCorrectData(t *testing.T) {
	w := Walker{
		cur: item{
			path: "/root/",
			err:  fmt.Errorf("This is an error"),
			entry: Entry{
				Name: "root",
				Size: 123,
				Time: time.Now(),
				Type: EntryTypeFolder,
			},
		},
	}

	assert.Equal(t, "This is an error", w.Err().Error())
	assert.Equal(t, "/root/", w.Path())
	assert.Equal(t, EntryTypeFolder, w.Stat().Type)
}

func TestSkipDirIsCorrectlySet(t *testing.T) {
	w := Walker{}

	w.SkipDir()

	assert.Equal(t, false, w.descend)
}

func TestNoDescendDoesNotAddToStack(t *testing.T) {
	w := new(Walker)
	w.cur = item{
		path: "/root/",
		err:  nil,
		entry: Entry{
			Name: "root",
			Size: 123,
			Time: time.Now(),
			Type: EntryTypeFolder,
		},
	}

	w.stack = []item{
		item{
			path: "file",
			err:  nil,
			entry: Entry{
				Name: "file",
				Size: 123,
				Time: time.Now(),
				Type: EntryTypeFile,
			},
		},
	}

	w.SkipDir()

	result := w.Step()

	assert.Equal(t, true, result, "Result should return true")
	assert.Equal(t, 0, len(w.stack))
	assert.Equal(t, true, w.descend)
}

func TestEmptyStackReturnsFalse(t *testing.T) {
	w := new(Walker)
	w.cur = item{
		path: "/root/",
		err:  nil,
		entry: Entry{
			Name: "root",
			Size: 123,
			Time: time.Now(),
			Type: EntryTypeFolder,
		},
	}

	w.stack = []item{}

	w.SkipDir()

	result := w.Step()

	assert.Equal(t, false, result, "Result should return false")
}

func TestCurAndStackSetCorrectly(t *testing.T) {
	w := new(Walker)
	w.cur = item{
		path: "root/file1",
		err:  nil,
		entry: Entry{
			Name: "file1",
			Size: 123,
			Time: time.Now(),
			Type: EntryTypeFile,
		},
	}

	w.stack = []item{
		item{
			path: "file",
			err:  nil,
			entry: Entry{
				Name: "file",
				Size: 123,
				Time: time.Now(),
				Type: EntryTypeFile,
			},
		},
		item{
			path: "root/file1",
			err:  nil,
			entry: Entry{
				Name: "file1",
				Size: 123,
				Time: time.Now(),
				Type: EntryTypeFile,
			},
		},
	}

	result := w.Step()
	result = w.Step()

	assert.Equal(t, true, result, "Result should return true")
	assert.Equal(t, 0, len(w.stack))
	assert.Equal(t, "file", w.cur.entry.Name)
}

func TestStackIsPopulatedCorrectly(t *testing.T) {

	mock, err := newFtpMock(t, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	c, cErr := Connect(mock.Addr())
	if cErr != nil {
		t.Fatal(err)
	}

	w := Walker{
		cur: item{
			path: "/root",
			entry: Entry{
				Name: "root",
				Size: 123,
				Time: time.Now(),
				Type: EntryTypeFolder,
			},
		},
		serverConn: c,
	}

	w.descend = true

	w.Step()

	assert.Equal(t, 0, len(w.stack))
	assert.Equal(t, "lo", w.cur.entry.Name)
	assert.Equal(t, true, strings.HasSuffix(w.cur.path, "/"))
}
