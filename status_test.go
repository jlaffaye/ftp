package ftp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusText(t *testing.T) {
	assert.Equal(t, "Unknown status code: 0", StatusText(0))
	assert.Equal(t, "Invalid username or password.", StatusText(StatusInvalidCredentials))
}
