//go:build linux

// RFC: rfc/short/rfc9568.md -- Section 7.3 / errata 7947/7949 (gratuitous ARP)
// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- AF_PACKET gratuitous-ARP sender on the macvlan
//
// The gratuitous-ARP announce socket is an AF_PACKET/SOCK_RAW socket bound to the
// macvlan ifindex (isis Send model). The frame built by garp.go already carries
// the Ethernet header (virtual-MAC source, broadcast destination) and is sent
// verbatim; the sockaddr only selects the egress interface.

package transport

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// openGARPSocket opens the AF_PACKET/SOCK_RAW ETH_P_ARP socket bound to the
// macvlan ifindex.
func openGARPSocket(macvlanIf int) (int, error) {
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ARP)))
	if err != nil {
		return -1, fmt.Errorf("vrrp/transport: socket(AF_PACKET, ARP) needs CAP_NET_RAW: %w", err)
	}
	sa := &unix.SockaddrLinklayer{Protocol: htons(unix.ETH_P_ARP), Ifindex: macvlanIf}
	if berr := unix.Bind(fd, sa); berr != nil {
		closeFD(fd)
		return -1, fmt.Errorf("vrrp/transport: bind(AF_PACKET) macvlan ifindex %d: %w", macvlanIf, berr)
	}
	return fd, nil
}

// sendGARPLocked transmits a prepared 42-byte gratuitous-ARP frame on the macvlan.
// li.sendMu must be held. RFC 9568 Section 7.3: the L2 source and ARP sha/tha are
// the Virtual Router MAC so bridges relearn the virtual MAC on the new Master.
func (li *linuxInstance) sendGARPLocked(frame []byte) error {
	var addr [8]byte
	copy(addr[:], broadcastMAC[:])
	sa := &unix.SockaddrLinklayer{Protocol: htons(unix.ETH_P_ARP), Ifindex: li.macvlanIf, Halen: 6, Addr: addr}
	if err := unix.Sendto(li.annFD, frame, 0, sa); err != nil {
		return fmt.Errorf("vrrp/transport: garp sendto: %w", err)
	}
	return nil
}
