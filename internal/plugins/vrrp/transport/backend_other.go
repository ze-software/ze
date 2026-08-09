//go:build !linux

// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- non-Linux backend stub (ospf backend_other.go model)
//
// VRRP's raw proto-112 sockets, AF_PACKET GARP, and raw ICMPv6 NA senders are
// Linux-only. Off Linux the backend returns a typed error so `make ze-verify`
// (darwin) still builds and config validation keeps working; the pure frame
// builders (garp.go / na.go) remain testable here.

package transport

import "errors"

// ErrUnsupportedPlatform is returned by the non-Linux backend: the raw VRRP
// transport needs Linux raw sockets.
var ErrUnsupportedPlatform = errors.New("vrrp/transport: raw VRRP transport is only supported on Linux")

type unsupportedBackend struct{}

// NewBackend returns the non-Linux stub backend.
func NewBackend() Backend { return unsupportedBackend{} }

func (unsupportedBackend) OpenInstance(InstanceSpec, rxSink) (InstanceHandle, error) {
	return nil, ErrUnsupportedPlatform
}
