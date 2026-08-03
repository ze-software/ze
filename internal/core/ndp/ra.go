// Design: docs/architecture/core-design.md -- shared IPv6 Neighbor Discovery encoding
//
// Package ndp encodes IPv6 Neighbor Discovery messages (RFC 4861) and the
// Recursive DNS Server option (RFC 8106). It lives in core so the LAN Router
// Advertisement sender and the L2TP/PPP subscriber path can each build the same
// wire format without depending on one another.
//
// The encoder is buffer-first: the caller owns the buffer, asks RALen for the
// encoded size, and gets the octet count back. Nothing here allocates.
package ndp

import (
	"encoding/binary"
	"net/netip"
)

// ICMPv6 and Neighbor Discovery option type numbers.
const (
	// ICMPv6TypeRouterSolicitation is the Router Solicitation type (RFC 4861 Section 4.1).
	ICMPv6TypeRouterSolicitation = 133
	// ICMPv6TypeRouterAdvertisement is the Router Advertisement type (RFC 4861 Section 4.2).
	ICMPv6TypeRouterAdvertisement = 134

	// OptSourceLinkLayerAddress is the Source Link-layer Address option (RFC 4861 Section 4.6.1).
	OptSourceLinkLayerAddress = 1
	// OptPrefixInformation is the Prefix Information option (RFC 4861 Section 4.6.2).
	OptPrefixInformation = 3
	// OptRDNSS is the Recursive DNS Server option (RFC 8106 Section 5.1).
	OptRDNSS = 25
)

// Router Advertisement flag bits in the octet after Cur Hop Limit
// (RFC 4861 Section 4.2). The remaining six bits are Reserved and are sent
// as zero.
const (
	raFlagManaged = 0x80
	raFlagOther   = 0x40
)

// Prefix Information option flag bits (RFC 4861 Section 4.6.2). The remaining
// six bits are Reserved1 and are sent as zero.
const (
	prefixFlagOnLink     = 0x80
	prefixFlagAutonomous = 0x40
)

// Encoded sizes in octets.
const (
	raHeaderLen      = 16
	prefixOptionLen  = 32
	rdnssOptionFixed = 8
	rdnssAddressLen  = 16
	// linkLayerOptionLen covers a type octet, a length octet, and a six-octet
	// IEEE 802 address: one 8-octet unit.
	linkLayerOptionLen = 8
	// linkLayerAddressLen is the only link-layer address length that fits
	// linkLayerOptionLen.
	linkLayerAddressLen = linkLayerOptionLen - 2
)

// LifetimeInfinity in a Prefix Information or RDNSS lifetime field means the
// information never expires (RFC 4861 Section 4.6.2, RFC 8106 Section 5.1).
const LifetimeInfinity uint32 = 0xffffffff

// PrefixInfo is one advertised Prefix Information option (RFC 4861 Section 4.6.2).
type PrefixInfo struct {
	// Prefix is the advertised prefix. Bits after the prefix length are masked
	// off before encoding, because the sender must initialize them to zero.
	Prefix netip.Prefix
	// OnLink sets the L flag: the prefix can be used for on-link determination.
	OnLink bool
	// Autonomous sets the A flag: the prefix can be used for stateless address
	// autoconfiguration (RFC 4862).
	Autonomous bool
	// ValidLifetime is the seconds the prefix stays valid for on-link
	// determination. LifetimeInfinity means it never expires.
	ValidLifetime uint32
	// PreferredLifetime is the seconds addresses formed from the prefix stay
	// preferred. RFC 4861 Section 4.6.2 forbids a value above ValidLifetime;
	// the caller validates that, because the encoder states the wire layout
	// and does not judge the configuration.
	PreferredLifetime uint32
}

// RAConfig holds every field of a Router Advertisement this encoder emits.
// The zero value encodes a minimal 16-octet RA with no options.
type RAConfig struct {
	// CurHopLimit is the default IP Hop Count for hosts on the link; 0 means
	// unspecified by this router.
	CurHopLimit uint8
	// Managed sets the M flag: addresses are available through DHCPv6.
	Managed bool
	// OtherConfig sets the O flag: other configuration is available through
	// DHCPv6.
	OtherConfig bool
	// RouterLifetime is the seconds this router is usable as a default router;
	// 0 means it is not a default router (RFC 4861 Section 4.2).
	RouterLifetime uint16
	// ReachableTime is the milliseconds a neighbor stays reachable after a
	// reachability confirmation; 0 means unspecified.
	ReachableTime uint32
	// RetransTimer is the milliseconds between retransmitted Neighbor
	// Solicitations; 0 means unspecified.
	RetransTimer uint32
	// SourceLinkLayerAddress is the sending interface's link-layer address. A
	// six-octet IEEE 802 address becomes a Source Link-layer Address option.
	// Any other length is omitted, because the option length field counts
	// whole 8-octet units and only the six-octet form fills exactly one.
	SourceLinkLayerAddress []byte
	// Prefixes are the advertised Prefix Information options, in order.
	Prefixes []PrefixInfo
	// RDNSS are recursive DNS server addresses advertised in one RFC 8106
	// option. They share RDNSSLifetime.
	RDNSS []netip.Addr
	// RDNSSLifetime is the seconds the resolvers can be used; 0 means they must
	// no longer be used (RFC 8106 Section 5.1).
	RDNSSLifetime uint32
}

// hasSourceLinkLayerAddress reports whether the configured link-layer address
// fits the single 8-octet unit this encoder emits.
func (c RAConfig) hasSourceLinkLayerAddress() bool {
	return len(c.SourceLinkLayerAddress) == linkLayerAddressLen
}

// RALen returns the exact number of octets BuildRA writes for cfg.
func RALen(cfg RAConfig) int {
	n := raHeaderLen
	if cfg.hasSourceLinkLayerAddress() {
		n += linkLayerOptionLen
	}
	n += prefixOptionLen * len(cfg.Prefixes)
	if len(cfg.RDNSS) > 0 {
		n += rdnssOptionFixed + rdnssAddressLen*len(cfg.RDNSS)
	}
	return n
}

// BuildRA writes a Router Advertisement into buf starting at off and returns
// the number of octets written. It returns 0 and writes nothing when off is
// negative or buf has fewer than RALen(cfg) octets left. An RA is never shorter
// than its 16-octet header, so 0 unambiguously means "not encoded" and the
// caller must treat it as an error rather than as an empty message.
//
// The Checksum field is left zero: callers send RAs on a raw ICMPv6 socket,
// where the kernel computes the ICMPv6 checksum.
//
// Options are written in the order Source Link-layer Address, Prefix
// Information, RDNSS. RFC 4861 Section 4.6 gives options no required order and
// tells receivers to ignore the ones they do not recognize.
func BuildRA(buf []byte, off int, cfg RAConfig) int {
	if off < 0 || len(buf)-off < RALen(cfg) {
		return 0
	}
	start := off

	// RFC 4861 Section 4.2: Router Advertisement message header.
	buf[off] = ICMPv6TypeRouterAdvertisement
	buf[off+1] = 0 // Code
	buf[off+2] = 0 // Checksum, computed by the kernel for raw sockets
	buf[off+3] = 0
	off += 4

	buf[off] = cfg.CurHopLimit
	off++

	// RFC 4861 Section 4.2: the M and O flags sit in the top two bits; the
	// low six bits are Reserved and MUST be initialized to zero by the sender.
	var flags uint8
	if cfg.Managed {
		flags |= raFlagManaged
	}
	if cfg.OtherConfig {
		flags |= raFlagOther
	}
	buf[off] = flags
	off++

	binary.BigEndian.PutUint16(buf[off:], cfg.RouterLifetime)
	off += 2
	binary.BigEndian.PutUint32(buf[off:], cfg.ReachableTime)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], cfg.RetransTimer)
	off += 4

	off = writeSourceLinkLayerAddress(buf, off, cfg)
	off = writePrefixOptions(buf, off, cfg)
	off = writeRDNSS(buf, off, cfg)

	return off - start
}

// writeSourceLinkLayerAddress appends the Source Link-layer Address option
// (RFC 4861 Section 4.6.1) and returns the new offset.
func writeSourceLinkLayerAddress(buf []byte, off int, cfg RAConfig) int {
	if !cfg.hasSourceLinkLayerAddress() {
		return off
	}
	buf[off] = OptSourceLinkLayerAddress
	// RFC 4861 Section 4.6.1: Length counts the type and length octets too, in
	// units of 8 octets.
	buf[off+1] = linkLayerOptionLen / 8
	copy(buf[off+2:off+linkLayerOptionLen], cfg.SourceLinkLayerAddress)
	return off + linkLayerOptionLen
}

// writePrefixOptions appends one Prefix Information option per configured
// prefix (RFC 4861 Section 4.6.2) and returns the new offset.
func writePrefixOptions(buf []byte, off int, cfg RAConfig) int {
	for _, p := range cfg.Prefixes {
		buf[off] = OptPrefixInformation
		buf[off+1] = prefixOptionLen / 8 // Length 4, in 8-octet units
		buf[off+2] = uint8(p.Prefix.Bits())

		// RFC 4861 Section 4.6.2: L and A flags sit in the top two bits;
		// Reserved1 (the low six) MUST be initialized to zero by the sender.
		var flags uint8
		if p.OnLink {
			flags |= prefixFlagOnLink
		}
		if p.Autonomous {
			flags |= prefixFlagAutonomous
		}
		buf[off+3] = flags

		binary.BigEndian.PutUint32(buf[off+4:], p.ValidLifetime)
		binary.BigEndian.PutUint32(buf[off+8:], p.PreferredLifetime)
		binary.BigEndian.PutUint32(buf[off+12:], 0) // Reserved2

		// RFC 4861 Section 4.6.2: the bits in the prefix after the prefix
		// length MUST be initialized to zero by the sender.
		addr := p.Prefix.Masked().Addr().As16()
		copy(buf[off+16:off+prefixOptionLen], addr[:])

		off += prefixOptionLen
	}
	return off
}

// writeRDNSS appends the Recursive DNS Server option (RFC 8106 Section 5.1)
// and returns the new offset.
func writeRDNSS(buf []byte, off int, cfg RAConfig) int {
	if len(cfg.RDNSS) == 0 {
		return off
	}
	buf[off] = OptRDNSS
	// RFC 8106 Section 5.1: Length in 8-octet units is 3 for one address and
	// grows by 2 for every additional address, so 1 + 2*count.
	buf[off+1] = uint8(1 + 2*len(cfg.RDNSS))
	binary.BigEndian.PutUint16(buf[off+2:], 0) // Reserved
	binary.BigEndian.PutUint32(buf[off+4:], cfg.RDNSSLifetime)
	off += rdnssOptionFixed

	for _, addr := range cfg.RDNSS {
		a := addr.As16()
		copy(buf[off:off+rdnssAddressLen], a[:])
		off += rdnssAddressLen
	}
	return off
}
