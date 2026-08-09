//go:build linux

// RFC: rfc/short/rfc9568.md -- Section 6.4.1/6.4.2 / 8.2.2 (unsolicited NA)
// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- raw ICMPv6 unsolicited-NA sender on the macvlan
// Related: backend_linux.go -- socket-set lifecycle; owns the linuxInstance this extends
//
// The unsolicited-NA announce socket is a raw AF_INET6/IPPROTO_ICMPV6 socket bound
// to the macvlan (SO_BINDTODEVICE) with IPV6_MULTICAST_HOPS 255. The message built
// by na.go is the ICMPv6 body only; the kernel builds the IPv6 header (source from
// the IPV6_PKTINFO cmsg, dst ff02::1, hop limit 255) and computes the ICMPv6
// checksum (RFC 3542 Section 3.1), which the golden test recomputes.

package transport

import (
	"errors"
	"fmt"
	"net/netip"

	"golang.org/x/sys/unix"
)

// openNASocket opens the raw ICMPv6 socket bound to the macvlan with the ND hop
// limit set.
func openNASocket(macvlanName string) (int, error) {
	fd, err := unix.Socket(unix.AF_INET6, unix.SOCK_RAW, unix.IPPROTO_ICMPV6)
	if err != nil {
		return -1, fmt.Errorf("vrrp/transport: socket(raw ICMPv6) needs CAP_NET_RAW: %w", err)
	}
	if berr := unix.SetsockoptString(fd, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, macvlanName); berr != nil {
		closeFD(fd)
		return -1, fmt.Errorf("vrrp/transport: bind NA socket to %s: %w", macvlanName, berr)
	}
	// RFC 4861 Section 7.1.2: ND messages MUST use hop limit 255 (holo bug 13 negative).
	if herr := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_HOPS, 255); herr != nil {
		closeFD(fd)
		return -1, fmt.Errorf("vrrp/transport: setsockopt NA IPV6_MULTICAST_HOPS: %w", herr)
	}
	return fd, nil
}

// sendNALocked transmits a prepared unsolicited NA ICMPv6 message to ff02::1 with
// the macvlan link-local as source (IPV6_PKTINFO). li.sendMu must be held. Returns
// ErrNoLinkLocal while the macvlan has no link-local yet.
func (li *linuxInstance) sendNALocked(frame []byte) error {
	src, ok := li.v6SourceLocked()
	if !ok {
		return ErrNoLinkLocal
	}
	oob := pktinfoOOB(src.As16(), li.macvlanIf)
	sa := &unix.SockaddrInet6{Addr: NAAllNodesV6.As16(), ZoneId: uint32(li.macvlanIf)}
	if err := unix.Sendmsg(li.annFD, frame, oob, sa, 0); err != nil {
		if errors.Is(err, unix.EINVAL) {
			// Pktinfo source no longer valid on the macvlan (removed/DAD-cycled);
			// drop the cache and report no-link-local so the counter reflects the
			// real condition and the next Master transition re-announces (R-2).
			li.v6Src = netip.Addr{}
			return ErrNoLinkLocal
		}
		return fmt.Errorf("vrrp/transport: na sendmsg: %w", err)
	}
	return nil
}
