// Design: docs/architecture/api/commands.md — shared ICMP probe primitives
//
// Package probe holds the low-level ICMP echo and target-resolution helpers
// shared by the active-probe commands (ping, traceroute, probe-round, and the
// tcp-check resolver). It lives in core so the ping and traceroute feature
// modules can each own their command surface without depending on one another
// or on the central show package for these primitives.
package probe

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
)

// ICMP raw-socket network identifiers for net.ListenConfig.ListenPacket.
const (
	NetworkICMPv4 = "ip4:icmp"
	NetworkICMPv6 = "ip6:ipv6-icmp"
)

// BuildICMPEcho builds an ICMP echo packet of the given type (8 for ICMPv4
// echo request, 128 for ICMPv6) with the given identifier, sequence number,
// and payload, and fills in the checksum.
func BuildICMPEcho(typ byte, id, seq uint16, data []byte) []byte {
	b := make([]byte, 8+len(data))
	b[0] = typ
	b[1] = 0
	binary.BigEndian.PutUint16(b[4:], id)
	binary.BigEndian.PutUint16(b[6:], seq)
	copy(b[8:], data)
	binary.BigEndian.PutUint16(b[2:], icmpChecksum(b))
	return b
}

// icmpChecksum computes the standard 16-bit one's-complement ICMP checksum.
func icmpChecksum(b []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(b[i:]))
	}
	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum += sum >> 16
	return ^uint16(sum)
}

// ResolveTarget parses s as an IP address, or resolves it as a hostname and
// returns the first address found.
func ResolveTarget(s string) (netip.Addr, error) {
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr, nil
	}
	ips, err := net.DefaultResolver.LookupNetIP(context.Background(), "ip", s)
	if err != nil {
		return netip.Addr{}, err
	}
	if len(ips) == 0 {
		return netip.Addr{}, fmt.Errorf("no addresses for %q", s)
	}
	return ips[0], nil
}
