//go:build !linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- non-Linux backend stub

package transport

import "errors"

var ErrUnsupportedPlatform = errors.New("ospf/transport: raw IPv4 transport is only supported on Linux")

type unsupportedBackend struct{}

func NewBackend() Backend { return unsupportedBackend{} }

func (unsupportedBackend) OpenInterface(string, dropRecorder) (InterfaceHandle, error) {
	return nil, ErrUnsupportedPlatform
}
