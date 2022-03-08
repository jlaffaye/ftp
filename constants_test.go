package ftp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusText(t *testing.T) {
	assert.Equal(t, "Unknown status code: 0", StatusText(0))
	assert.Equal(t, "Invalid username or password.", StatusText(StatusInvalidCredentials))
}

func TestEntryTypeString(t *testing.T) {
	assert.Equal(t, "file", EntryTypeFile.String())
	assert.Equal(t, "folder", EntryTypeFolder.String())
	assert.Equal(t, "link", EntryTypeLink.String())
}
