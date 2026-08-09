//go:build !linux

// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- non-Linux backend stub
// RFC: rfc/short/rfc5340.md (§2.9 raw IPv6 transport is a Linux kernel capability)

package transport

import "errors"

// ErrUnsupportedPlatform reports that the raw IPv6 OSPFv3 transport (raw proto-89
// socket + IPv6 multicast membership) is only implemented on Linux.
var ErrUnsupportedPlatform = errors.New("ospfv3/transport: raw IPv6 transport is only supported on Linux")

// NewBackend returns the platform backend. On non-Linux platforms it reports the
// raw IPv6 socket + multicast path is unavailable, so config and unit tests still
// build and run.
func NewBackend() Backend { return unsupportedBackend{} }

type unsupportedBackend struct{}

func (unsupportedBackend) OpenInterface(string, DropRecorder) (InterfaceHandle, error) {
	return nil, ErrUnsupportedPlatform
}
