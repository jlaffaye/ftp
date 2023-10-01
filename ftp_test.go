package ftp

import (
	"net"
	"testing"
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
