package ftp

import "io"

type listingLimitReader struct {
	reader    io.Reader
	remaining int64
}

func (r *listingLimitReader) Read(p []byte) (int, error) {
	if int64(len(p)) > r.remaining {
		p = p[:int(r.remaining)+1]
	}
	n, err := r.reader.Read(p)
	if int64(n) > r.remaining {
		return n, ErrListingLimit
	}
	r.remaining -= int64(n)
	return n, err
}
