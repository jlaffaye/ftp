package ftp

import (
	"errors"
	"strings"
)

// mergeErrors into one, nil errors are discarded
// and returns nil if all errors are nil.
func mergeErrors(err ...error) error {
	var errs []error
	for _, e := range err {
		if e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return &mergedError{s: errs}
}

type mergedError struct{ s []error }

var _ error = (*mergedError)(nil)

// Error implements the error interface and concatenates
// all errors, separated by ": ".
func (e *mergedError) Error() string {
	var s strings.Builder
	for i, err := range e.s {
		if i > 0 {
			s.WriteString(": ")
		}
		s.WriteString(err.Error())
	}
	return s.String()
}

// Unwrap returns only the first error as there is
// no way to create a queue of errors.
func (e *mergedError) Unwrap() error { return e.s[0] }

// Is does errors.Is on all merged errors.
func (e *mergedError) Is(target error) bool {
	if target == nil {
		return nil == e.s
	}
	for _, err := range e.s {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// As does errors.As on all merged errors.
func (e *mergedError) As(target interface{}) bool {
	for _, err := range e.s {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}

// handleClose can be used to defer handling of io.Closer Close
// functions. Close will always be called. If Close returns an
// error, it will be merged with dst. The calling function must
// use a named error return and pass the pointer as dst.
func handleClose(dst *error, close func() error) {
	err := close()
	if err != nil {
		if *dst == nil {
			*dst = err
			return
		}
		*dst = mergeErrors(*dst, err)
	}
}

// handleCloserOnError can be used to defer handling of io.Closer
// Close functions where Close should only be called on error.
// If Close returns an error, it will be merged with dst. The
// calling function must use a named error return  and pass the
// pointer as dst.
func handleCloseOnError(dst *error, close func() error) {
	if *dst != nil {
		err := close()
		if err != nil {
			*dst = mergeErrors(*dst, err)
		}
	}
}
