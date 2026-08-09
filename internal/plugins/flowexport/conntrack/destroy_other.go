//go:build !linux

// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- conntrack destroy listener stub
// Related: destroy.go -- ctnetlink event parser (shared, platform-independent)

package conntrack

// DestroyListener is a stub for non-Linux platforms. The ctnetlink multicast
// subscription is Linux-only; the parser in destroy.go remains shared so it can
// be unit-tested on any platform.
type DestroyListener struct{}

// NewDestroyListener returns an error on non-Linux platforms.
func NewDestroyListener() (*DestroyListener, error) {
	return nil, errNotSupported
}

// Read is not supported on non-Linux platforms.
func (l *DestroyListener) Read() ([]FlowEntry, error) {
	return nil, errNotSupported
}

// Close is a no-op on non-Linux platforms.
func (l *DestroyListener) Close() error {
	return nil
}
