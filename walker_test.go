package ftp

import (
	"fmt"
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
	assert.Equal(t, 1, len(w.stack))
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

func TestErrorsFromListAreHandledCorrectly(t *testing.T) {
	//Get error
	//Check w.cur.err
	//Check stack
}

func TestStackIsPopulatedCorrectly(t *testing.T) {
	//Check things are added to the stack correcty
}
