# goftp #

[![Build Status](https://travis-ci.org/jlaffaye/ftp.svg?branch=master)](https://travis-ci.org/jlaffaye/ftp)
[![Coverage Status](https://coveralls.io/repos/jlaffaye/ftp/badge.svg?branch=master&service=github)](https://coveralls.io/github/jlaffaye/ftp?branch=master)
[![Go ReportCard](http://goreportcard.com/badge/jlaffaye/ftp)](http://goreportcard.com/report/jlaffaye/ftp)

A FTP client package for Go

## Install ##

```
go get -u github.com/jlaffaye/ftp
```

## Documentation ##

http://godoc.org/github.com/jlaffaye/ftp

## Concurrency Notes ##

`ServerConn` is safe for concurrent access. What this means in practice is that
you can dial a connection to your FTP server, login, and pass around the
resulting `ServerConn` to multiple goroutines. However, this does not mean that
you can simultaneously upload or download multiple files. Because of limitations
inherent to the FTP protocol, there is a lock around these kinds of methods. The
user also needs to be aware that any call to `ChangeDir()` or similar will be
felt across every goroutine accessing that session. It is best to handle those
calls synchronously, eg immediately after logging into the FTP server.
