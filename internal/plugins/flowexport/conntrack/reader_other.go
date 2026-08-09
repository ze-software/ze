// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- Conntrack reader stub for non-Linux

//go:build !linux

package conntrack

import "errors"

var errNotSupported = errors.New("conntrack: not supported on this platform")

// Reader is a stub for non-Linux platforms.
type Reader struct{}

// NewReader returns an error on non-Linux platforms.
func NewReader() (*Reader, error) {
	return nil, errNotSupported
}

// Dump is not supported on non-Linux platforms.
func (r *Reader) Dump() ([]FlowEntry, error) {
	return nil, errNotSupported
}

// Close is a no-op on non-Linux platforms.
func (r *Reader) Close() error {
	return nil
}
