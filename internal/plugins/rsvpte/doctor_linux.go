//go:build linux

// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- raw-socket probe for the doctor check
// Related: transport_linux.go -- the production raw socket this mirrors

package rsvpte

import "golang.org/x/sys/unix"

// rsvpRawSocketAvailable reports whether a raw IP socket for protocol 46 can be
// opened (the capability RSVP-TE's transport needs). It opens and immediately
// closes the socket; EPERM (no CAP_NET_RAW) makes it return false.
func rsvpRawSocketAvailable() bool {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, rsvpProtocol)
	if err != nil {
		return false
	}
	if cerr := unix.Close(fd); cerr != nil {
		logger().Warn("rsvp-te: close raw-socket probe", "err", cerr)
	}
	return true
}
