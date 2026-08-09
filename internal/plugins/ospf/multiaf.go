// Design: docs/architecture/ospf/ospf-ext-15-multi-af.md -- RFC 5838 multiple address families over OSPFv3.
// Related: dispatcher.go -- the per-instance Instance-ID demux (RFC 5340 §4.2.2) reused per AF.
// Related: register.go -- one v6-codec engine spawned per configured AF.
//
// RFC 5838 carries several address families over the single OSPFv3 (IPv6) wire by mapping
// each AF to a reserved Instance-ID range and tagging its packets with the AF-bit in the
// Options field. This file owns the AF <-> Instance-ID-range mapping (§2.1), the per-AF
// Loc-RIB install family, the prefix address width, and the AF-bit emission rule (§2.5/§2.6).
//
// RFC: rfc/short/rfc5838.md
package ospf

import (
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// dropReasonAFBit labels a Hello dropped because a non-default OSPFv3 address family
// received it without the RFC 5838 §2.5 AF-bit set (the neighbor is not brought to Full).
const dropReasonAFBit = "af-bit"

// addressFamily is an OSPFv3 address family selected by its Instance-ID range (RFC 5838
// §2.1). It is a property of one v6-codec engine instance, derived once from the engine's
// configured Instance ID.
type addressFamily uint8

const (
	// afIPv6Unicast is the default OSPFv3 address family (RFC 5838 §2.1: Instance ID 0-31).
	afIPv6Unicast addressFamily = iota
	// afIPv6Multicast maps to Instance ID 32-63 (RFC 5838 §2.1).
	afIPv6Multicast
	// afIPv4Unicast maps to Instance ID 64-95 (RFC 5838 §2.1): IPv4-over-OSPFv3.
	afIPv4Unicast
	// afIPv4Multicast maps to Instance ID 96-127 (RFC 5838 §2.1).
	afIPv4Multicast
)

// AF name spellings shared by String, afFromName, and the config parser.
const (
	afNameIPv6Unicast   = "ipv6-unicast"
	afNameIPv6Multicast = "ipv6-multicast"
	afNameIPv4Unicast   = "ipv4-unicast"
	afNameIPv4Multicast = "ipv4-multicast"
	// afNameIPv6 is the bare spelling of the default (IPv6-unicast) address family.
	afNameIPv6 = "ipv6"
)

// RFC 5838 §2.1 Instance-ID range bounds (inclusive) per address family.
const (
	afIPv6UnicastMin, afIPv6UnicastMax     uint8 = 0, 31
	afIPv6MulticastMin, afIPv6MulticastMax uint8 = 32, 63
	afIPv4UnicastMin, afIPv4UnicastMax     uint8 = 64, 95
	afIPv4MulticastMin, afIPv4MulticastMax uint8 = 96, 127
	// afInstanceIDMax is the largest Instance ID usable for AF mapping; RFC 5838 §2.1
	// reserves 0-127. 128-255 is invalid for AF use.
	afInstanceIDMax uint8 = 127
)

// afFromInstanceID maps an OSPFv3 Instance ID to its RFC 5838 §2.1 address family. ok is
// false for an Instance ID above 127 (outside the AF-usable space).
func afFromInstanceID(id uint8) (addressFamily, bool) {
	switch {
	case id <= afIPv6UnicastMax:
		return afIPv6Unicast, true
	case id <= afIPv6MulticastMax:
		return afIPv6Multicast, true
	case id <= afIPv4UnicastMax:
		return afIPv4Unicast, true
	case id <= afIPv4MulticastMax:
		return afIPv4Multicast, true
	default:
		return 0, false
	}
}

// afInstanceIDRange returns the inclusive Instance-ID range for an address family
// (RFC 5838 §2.1).
func afInstanceIDRange(af addressFamily) (min, max uint8) {
	switch af {
	case afIPv6Unicast:
		return afIPv6UnicastMin, afIPv6UnicastMax
	case afIPv6Multicast:
		return afIPv6MulticastMin, afIPv6MulticastMax
	case afIPv4Unicast:
		return afIPv4UnicastMin, afIPv4UnicastMax
	case afIPv4Multicast:
		return afIPv4MulticastMin, afIPv4MulticastMax
	default:
		return 0, afInstanceIDMax
	}
}

// afInstanceIDInRange reports whether id falls inside af's RFC 5838 §2.1 Instance-ID range.
func afInstanceIDInRange(af addressFamily, id uint8) bool {
	min, max := afInstanceIDRange(af)
	return id >= min && id <= max
}

// family returns the Loc-RIB family an AF's SPF routes install into. This is the
// parameter that fixes the IPv6-base install-family hardcode (RFC 5838: "Address family
// determines Loc-RIB family").
func (af addressFamily) family() family.Family {
	switch af {
	case afIPv6Unicast:
		return family.IPv6Unicast
	case afIPv6Multicast:
		return family.IPv6Multicast
	case afIPv4Unicast:
		return family.IPv4Unicast
	case afIPv4Multicast:
		return family.IPv4Multicast
	default:
		return family.IPv6Unicast
	}
}

// isIPv4 reports whether the AF carries IPv4 prefixes (4-byte address width) rather than
// IPv6 (16-byte). RFC 5838 §2.7: an IPv4 prefix rides the RFC 5340 prefix codec as a
// single 32-bit word.
func (af addressFamily) isIPv4() bool {
	return af == afIPv4Unicast || af == afIPv4Multicast
}

// prefixWidth returns the AF's address byte width: 4 for IPv4 families, 16 for IPv6.
func (af addressFamily) prefixWidth() int {
	if af.isIPv4() {
		return 4
	}
	return 16
}

// isDefault reports whether this is the default IPv6-unicast address family. RFC 5838 §2.6
// treats the default AF specially: it still forms an adjacency with a neighbor that omits
// the AF-bit, whereas every other AF requires the AF-bit for the adjacency (§2.5).
func (af addressFamily) isDefault() bool { return af == afIPv6Unicast }

// String renders the AF as its kebab-case name (metrics/show labels).
func (af addressFamily) String() string {
	switch af {
	case afIPv6Unicast:
		return afNameIPv6Unicast
	case afIPv6Multicast:
		return afNameIPv6Multicast
	case afIPv4Unicast:
		return afNameIPv4Unicast
	case afIPv4Multicast:
		return afNameIPv4Multicast
	default:
		return "unknown-af"
	}
}

// installFamily returns the Loc-RIB family this engine's SPF routes install into. The
// OSPFv2 (v4Codec) engine always installs into IPv4-unicast; a v6-codec engine installs
// into its RFC 5838 address family, which also corrects the IPv6-base IPv4-unicast hardcode.
func (e *engine) installFamily() family.Family {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return e.af.family()
	}
	return family.IPv4Unicast
}

// emitAFBit reports whether this engine sets the AF-bit in originated Hello/DD Options.
// RFC 5838 §2.4/§2.5: a non-default AF always sets it; the default IPv6-unicast AF sets it
// only when the router is multi-AF-aware, so a lone IPv6-unicast instance keeps the
// IPv6-base wire bytes (AC-11). The OSPFv2 (v4Codec) encoder ignores the flag.
func (e *engine) emitAFBit() bool {
	return !e.af.isDefault() || e.multiAF.Load()
}

// setMultiAF records whether the router runs more than one OSPFv3 address family. Register
// calls it before setConfig so the default instance's AF-bit emission is correct.
func (e *engine) setMultiAF(b bool) { e.multiAF.Store(b) }

// afBitAccepted is the shared RFC 5838 §2.5/§2.6 AF-bit adjacency gate applied to a received
// Hello or Database Description. A non-default AF requires the AF-bit: a neighbor whose
// Options lack it is not brought up (the mismatch is counted). The default IPv6-unicast AF
// ignores a missing AF-bit (§2.6), so it still peers with a legacy neighbor. OSPFv2 (no AF
// concept) always accepts.
func (e *engine) afBitAccepted(afBit bool) bool {
	if e.dispatch == nil || !e.dispatch.codec.IsV6() || e.af.isDefault() {
		return true
	}
	if afBit {
		return true
	}
	e.mAFBitMismatch.With(e.af.String()).Inc()
	return false
}

// afHelloAccepted applies the AF-bit gate to a received Hello (RFC 5838 §2.5/§2.6).
func (e *engine) afHelloAccepted(h types.Hello) bool { return e.afBitAccepted(h.AFBit) }

// afDBDescAccepted applies the AF-bit gate to a received Database Description, so a neighbor
// that omits the AF-bit is never brought to Full even if it reaches the DD exchange (RFC 5838
// §2.5/§2.6 defense-in-depth beyond the Hello gate).
func (e *engine) afDBDescAccepted(d types.DBDesc) bool { return e.afBitAccepted(d.AFBit) }

// afFromName maps a configured address-family name to its AF. The bare "ipv6" spelling is
// the IPv6-unicast default AF. ok is false for an unknown name.
func afFromName(name string) (addressFamily, bool) {
	switch name {
	case afNameIPv6, afNameIPv6Unicast:
		return afIPv6Unicast, true
	case afNameIPv6Multicast:
		return afIPv6Multicast, true
	case afNameIPv4Unicast:
		return afIPv4Unicast, true
	case afNameIPv4Multicast:
		return afIPv4Multicast, true
	default:
		return 0, false
	}
}
