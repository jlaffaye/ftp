package ftp

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBogusDataIP(t *testing.T) {
	for _, tC := range []struct {
		cmd, data net.IP
		bogus     bool
	}{
		{net.IPv4(192, 168, 1, 1), net.IPv4(192, 168, 1, 1), false},
		{net.IPv4(192, 168, 1, 1), net.IPv4(1, 1, 1, 1), true},
		{net.IPv4(10, 65, 1, 1), net.IPv4(1, 1, 1, 1), true},
		{net.IPv4(10, 65, 25, 1), net.IPv4(10, 65, 8, 1), false},
	} {
		if got, want := isBogusDataIP(tC.cmd, tC.data), tC.bogus; got != want {
			t.Errorf("%s,%s got %t, wanted %t", tC.cmd, tC.data, got, want)
		}
	}
}

func TestEPSV_Parse_Valid(t *testing.T) {
	port, err := parseEPSV("Entering Extended Passive Mode (|||4242|)")
	assert.NoError(t, err)
	assert.Equal(t, 4242, port)
}

func TestEPSV_Parse_MissingTrailingPipe_ShouldError(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("epsv() panicked, want graceful error: %v", r)
		}
	}()
	_, err := parseEPSV("Entering Extended Passive Mode (|||4242)")
	if err == nil {
		t.Fatalf("expected error for malformed EPSV response, got nil")
	}
}

func TestEPSV_Parse_MissingPortBetweenPipes_ShouldError(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("epsv() panicked, want graceful error: %v", r)
		}
	}()
	_, err := parseEPSV("Entering Extended Passive Mode (||||)")
	if err == nil {
		t.Fatalf("expected error for malformed EPSV response, got nil")
	}
}
