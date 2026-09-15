//go:build !linux

// Design: docs/architecture/diagnostics/active-probes.md -- the DF mode off Linux
// Related: socket_linux.go -- the Linux half with the same signature
// Related: socket.go -- OpenICMP, the caller
//
// A platform without IP_MTU_DISCOVER cannot set the DF bit through the socket
// layer. The stub keeps the package compiling there, and it names the absence:
// a mode that sets DF is refused with ErrDFUnsupported, never opened with the
// bit silently clear. DFOff installs nothing, so a probe with DF off opens
// exactly as it did before the mode existed.

package probe

import (
	"net"
	"net/netip"
	"syscall"
)

// dfControl reports the capability absent for a mode that sets DF, and a
// Control that sets nothing for DFOff: there is no IP_MTU_DISCOVER to clear
// here, so the socket opens exactly as it did before the mode existed. The
// signature matches socket_linux.go so OpenICMP is written once.
func dfControl(_ Family, df DFMode) (func(network, address string, c syscall.RawConn) error, error) {
	if df == DFOff {
		return func(_, _ string, _ syscall.RawConn) error { return nil }, nil
	}
	return nil, ErrDFUnsupported
}

// listenDatagramICMP reports the unprivileged fallback absent: no platform
// Ze builds for other than Linux offers the ping socket, so a raw refusal
// here is reported with both reasons and no socket is opened.
func listenDatagramICMP(_ Family, _ netip.Addr, _ DFMode) (net.PacketConn, uint16, error) {
	return nil, 0, ErrDatagramICMPUnsupported
}
