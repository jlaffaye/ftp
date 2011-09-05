package ftp

import "testing"

type line struct {
	line string
	name string
	entryType EntryType
}

var listTests = []line {
	// UNIX ls -l style
	line{"drwxr-xr-x    3 110      1002            3 Dec 02  2009 pub", "pub", EntryTypeFolder},
	line{"drwxr-xr-x    3 110      1002            3 Dec 02  2009 p u b", "p u b", EntryTypeFolder},
	line{"-rwxr-xr-x    3 110      1002            1234567 Dec 02  2009 fileName", "fileName", EntryTypeFile},
	line{"lrwxrwxrwx   1 root     other          7 Jan 25 00:17 bin -> usr/bin", "bin -> usr/bin", EntryTypeLink},
	// Microsoft's FTP servers for Windows
	line{"----------   1 owner    group         1803128 Jul 10 10:18 ls-lR.Z", "ls-lR.Z", EntryTypeFile},
	line{"d---------   1 owner    group               0 May  9 19:45 Softlib", "Softlib", EntryTypeFolder},
	// WFTPD for MSDOS
	line{"-rwxrwxrwx   1 noone    nogroup      322 Aug 19  1996 message.ftp", "message.ftp", EntryTypeFile},
}

// Not supported, at least we should properly return failure
var listTestsFail = []line {
	line{"d [R----F--] supervisor            512       Jan 16 18:53    login", "login", EntryTypeFolder},
	line{"- [R----F--] rhesus             214059       Oct 20 15:27    cx.exe", "cx.exe", EntryTypeFile},
}

func TestParseListLine(t *testing.T) {
	for _, lt := range listTests {
		entry, err := parseListLine(lt.line)
		if err != nil {
			t.Errorf("parseListLine(%v) returned err = %v", lt.line, err)
			continue
		}
		if entry.Name != lt.name {
			t.Errorf("parseListLine(%v).Name = '%v', want '%v'", lt.line, entry.Name, lt.name)
		}
		if entry.Type != lt.entryType {
			t.Errorf("parseListLine(%v).EntryType = %v, want %v", lt.line, entry.Type, lt.entryType,)
		}
	}
	for _, lt := range listTestsFail {
		_, err := parseListLine(lt.line)
		if err == nil {
			t.Errorf("parseListLine(%v) expected to fail", lt.line)
		}
	}
}
