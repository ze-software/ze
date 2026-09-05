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
	"errors"
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

// Family is the IP address family a target resolution is held to. A probe
// command derives it from the source address the operator gave, because a
// source can only be bound on a socket of its own family. The zero value holds
// the resolution to no family at all, which is what a command with no source
// address needs.
type Family uint8

const (
	// FamilyAny places no constraint: the first address of either family answers.
	FamilyAny Family = iota
	FamilyIPv4
	FamilyIPv6
)

// String names the family for an operator reading an error message.
func (f Family) String() string {
	switch f {
	case FamilyIPv4:
		return "IPv4"
	case FamilyIPv6:
		return "IPv6"
	default:
		return "any"
	}
}

// network is the net.Resolver network name that holds a lookup to f.
func (f Family) network() string {
	switch f {
	case FamilyIPv4:
		return "ip4"
	case FamilyIPv6:
		return "ip6"
	default:
		return "ip"
	}
}

// holds reports whether addr belongs to f. FamilyAny holds every valid address.
func (f Family) holds(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	if f == FamilyAny {
		return true
	}
	return FamilyOf(addr) == f
}

// FamilyOf is the family of addr, and FamilyAny for the zero address. An
// IPv4-mapped IPv6 address is IPv4, because that is the socket family it
// reaches the wire on.
func FamilyOf(addr netip.Addr) Family {
	if !addr.IsValid() {
		return FamilyAny
	}
	if addr.Unmap().Is4() {
		return FamilyIPv4
	}
	return FamilyIPv6
}

// ErrFamilyMismatch reports that the target carries no address in the family
// the caller asked for. A caller that named a family (because the operator gave
// a source address) tells the operator the two arguments conflict, rather than
// letting the conflict surface later as a socket bind failure naming neither.
var ErrFamilyMismatch = errors.New("no address in the requested family")

// ResolveTarget parses s as an IP address, or resolves it as a hostname and
// returns the first address found. family holds both routes to one address
// family; a target with no address in it returns ErrFamilyMismatch. An
// IPv4-mapped IPv6 literal is unmapped, so the address that comes back always
// names the socket family the probe opens.
func ResolveTarget(s string, family Family) (netip.Addr, error) {
	if addr, err := netip.ParseAddr(s); err == nil {
		addr = addr.Unmap()
		if !family.holds(addr) {
			return netip.Addr{}, familyMismatch(s)
		}
		return addr, nil
	}
	ips, err := net.DefaultResolver.LookupNetIP(context.Background(), family.network(), s)
	if err != nil {
		if family != FamilyAny && familyAbsent(err) {
			return netip.Addr{}, familyMismatch(s)
		}
		return netip.Addr{}, err
	}
	if len(ips) == 0 {
		if family != FamilyAny {
			return netip.Addr{}, familyMismatch(s)
		}
		return netip.Addr{}, fmt.Errorf("no addresses for %q", s)
	}
	return ips[0].Unmap(), nil
}

// familyAbsent reports whether a family-constrained lookup failed because the
// target carries no address in that family. LookupNetIP answers that in two
// shapes: a name whose records are all of the other family gives *net.AddrError
// ("no suitable address found"), and a name with no record at all in the asked
// family gives *net.DNSError with IsNotFound. Every other failure (SERVFAIL, a
// timeout, a refused query) says nothing about the family, so the caller
// reports it as itself rather than blaming the source address for it.
func familyAbsent(err error) bool {
	if _, ok := errors.AsType[*net.AddrError](err); ok {
		return true
	}
	dnsErr, ok := errors.AsType[*net.DNSError](err)
	return ok && dnsErr.IsNotFound
}

// familyMismatch is the one error every family conflict answers with, so a
// literal target and a resolved hostname are classified the same way. The
// caller names the family it asked for, and the source address behind it, in
// the message it writes for the operator.
func familyMismatch(s string) error {
	return fmt.Errorf("%q: %w", s, ErrFamilyMismatch)
}
