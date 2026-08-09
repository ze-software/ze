//go:build !linux

// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- non-Linux RSVP transport stub
// Related: transport.go -- Transport interface and Packet type
// Related: transport_linux.go -- the real raw-socket implementation
//
// RSVP's raw IP protocol-46 socket is Linux-specific (the gokrazy appliance
// target is Linux). On other platforms opening the transport fails cleanly so
// the component can still load for config parsing and unit tests without a
// privileged socket.
package rsvpte

import (
	"errors"
	"net/netip"
	"runtime"
)

var errTransportUnsupported = errors.New("rsvp-te: raw IP transport unsupported on " + runtime.GOOS)

func openRawTransport(_ netip.Addr) (Transport, error) {
	return nil, errTransportUnsupported
}
