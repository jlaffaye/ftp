package ftp

import (
	"errors"
	"io"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

func listingTestClient(t *testing.T, listing string, mlsd bool) *ServerConn {
	t.Helper()
	control, peer := net.Pipe()
	data, dataPeer := net.Pipe()
	for _, conn := range []net.Conn{control, peer, data, dataPeer} {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		tp := textproto.NewConn(peer)
		defer tp.Close()
		if _, err := tp.ReadLine(); err != nil {
			return
		}
		if err := tp.PrintfLine("229 Entering Extended Passive Mode (|||2121|)"); err != nil {
			return
		}
		command, err := tp.ReadLine()
		if err != nil {
			return
		}
		want := "LIST /"
		if mlsd {
			want = "MLSD /"
		}
		if command != want {
			t.Errorf("command = %q, want %q", command, want)
		}
		if err := tp.PrintfLine("150 opening"); err != nil {
			return
		}
		_, _ = io.WriteString(dataPeer, listing)
		_ = dataPeer.Close()
		_ = tp.PrintfLine("226 complete")
	}()
	t.Cleanup(func() {
		_ = control.Close()
		_ = peer.Close()
		_ = data.Close()
		_ = dataPeer.Close()
		<-done
	})
	return &ServerConn{
		conn: textproto.NewConn(control), netConn: control, host: "127.0.0.1", mlstSupported: mlsd,
		options: &dialOptions{dialFunc: func(string, string) (net.Conn, error) { return data, nil }, location: time.UTC},
	}
}

func TestListWithLimits(t *testing.T) {
	row := "-rw-r--r-- 1 u g 1 Oct 19 12:00 file.txt\r\n"
	mlsd := "type=file;size=1;modify=20261019120000; file.txt\r\n"
	for _, tc := range []struct {
		name                 string
		listing              string
		maxEntries, maxBytes int
		mlsd                 bool
		wantLimit            bool
		wantEntries          int
	}{
		{"entry limit", strings.Repeat(row, 3), 2, 4096, false, true, 0},
		{"byte limit", row, 100, len(row) - 1, false, true, 0},
		{"exact limits", row, 1, len(row), false, false, 1},
		{"empty listing", "", 1, 1, false, false, 0},
		{"unterminated line", strings.TrimRight(row, "\r\n"), 1, len(row), false, false, 1},
		{"invalid records count", "invalid\r\ninvalid\r\n" + row, 2, 4096, false, true, 0},
		{"CRLF bytes count", row, 1, len(row) - 2, false, true, 0},
		{"MLSD limit", strings.Repeat(mlsd, 3), 2, 4096, true, true, 0},
		{"MLSD success", mlsd, 1, len(mlsd), true, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := listingTestClient(t, tc.listing, tc.mlsd)
			entries, err := client.ListWithLimits("/", tc.maxEntries, tc.maxBytes)
			if tc.wantLimit {
				if !errors.Is(err, ErrListingLimit) || entries != nil {
					t.Fatalf("got %v, %v, want nil, ErrListingLimit", entries, err)
				}
				if _, err := client.netConn.Write([]byte("NOOP\r\n")); err == nil {
					t.Fatal("control connection remains open after listing limit")
				}
				return
			}
			if err != nil || len(entries) != tc.wantEntries {
				t.Fatalf("got %d entries, %v, want %d entries", len(entries), err, tc.wantEntries)
			}
		})
	}
}

func TestListWithLimitsRejectsInvalidLimitsBeforeConnecting(t *testing.T) {
	for _, limits := range [][2]int{{0, 1}, {1, 0}, {-1, 1}, {1, -1}} {
		var client *ServerConn
		if _, err := client.ListWithLimits("/", limits[0], limits[1]); err == nil {
			t.Fatalf("accepted limits %v", limits)
		}
	}
}

func TestListPreservesUnlimitedBehavior(t *testing.T) {
	client := listingTestClient(t, "-rw-r--r-- 1 u g 1 Oct 19 12:00 file.txt\r\n", false)
	entries, err := client.List("/")
	if err != nil || len(entries) != 1 {
		t.Fatalf("got %v, %v", entries, err)
	}
}
