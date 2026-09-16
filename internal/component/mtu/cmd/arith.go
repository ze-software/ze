// Design: docs/architecture/diagnostics/path-mtu.md -- the ceiling, the recommended value and the MSS
// RFC: rfc/short/rfc4303.md -- the ESP alignment and trailer the ceiling subtracts
// Related: overhead.go -- the per-transform overhead this arithmetic consumes
// Related: verdict.go -- the classification of a tunnel against these figures

package cmd

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/core/ipsecinventory"
)

// ipFamily is the address family of the packets INSIDE a tunnel, which decides
// the minimum link MTU a recommendation must clear and the header the MSS
// subtracts. The inventory's Tunnel carries no traffic selector, so the
// caller supplies it. The zero value is Unspecified so an unset family never
// passes for one.
type ipFamily uint8

const (
	ipFamilyUnspecified ipFamily = iota
	ipFamilyV4
	ipFamilyV6
)

// familyOf answers the family of one address. An invalid address answers
// Unspecified.
func familyOf(addr netip.Addr) ipFamily {
	if !addr.IsValid() {
		return ipFamilyUnspecified
	}
	if addr.Is6() {
		return ipFamilyV6
	}
	return ipFamilyV4
}

// String is for the payload only; never compare with it.
func (f ipFamily) String() string {
	switch f {
	case ipFamilyV4:
		return "ipv4"
	case ipFamilyV6:
		return "ipv6"
	default:
		panic("BUG: ipFamily written to the payload before it was set")
	}
}

const (
	// ipv6MinimumLinkMTU is the floor below which no IPv6-carrying tunnel may
	// be sized. RFC 8200 Section 5: "IPv6 requires that every link in the
	// Internet have an MTU of 1280 octets or greater. This is known as the
	// IPv6 minimum link MTU", and a tunnel is a link.
	ipv6MinimumLinkMTU = 1280

	// ipv4MinimumMTU is the floor this diagnostic applies to an IPv4-carrying
	// tunnel. It is a documented default carried over from the ported tool,
	// not a conformance claim: RFC 791 Section 3.2 makes 576 a reassembly
	// minimum ("All hosts must be prepared to accept datagrams of up to 576
	// octets (whether they arrive whole or in fragments)"), and no RFC names
	// an IPv4 minimum link MTU (docs/architecture/diagnostics/path-mtu.md,
	// "The arithmetic").
	ipv4MinimumMTU = 576

	// recommendedMarginOctets is how far the recommended MTU backs off from
	// the ceiling. The ported tool's reason, kept verbatim: "safety margin;
	// the access circuit is not ours". The figure is pinned because operators
	// already rely on the verdict it produces (A-5).
	recommendedMarginOctets = 32
)

// errNoUsableMTU is the answer when the ceiling leaves no value at or above
// the inner family's minimum. It is a named outcome rather than a zero: a zero
// recommendation reads as a value and would reach a `set ... mtu 0` command.
var errNoUsableMTU = errors.New("mtu: no usable tunnel MTU")

// alignDown rounds value down to a multiple of block. A block of zero is a
// programmer error: every espOverhead carries a block of at least 4.
func alignDown(value, block uint16) uint16 {
	if block == 0 {
		panic("BUG: alignDown with a zero block")
	}
	return value - value%block
}

// ceiling is the largest packet a Child SA can be handed so that, after ESP,
// it fits the underlay path MTU. It is 0 when nothing is left, and 0 is a
// legitimate figure here: recommended turns it into errNoUsableMTU.
//
// Tunnel mode: the packet is the ESP payload, so the outer header and the
// fixed octets come off the underlay, what remains is padded down to the
// cipher block, and the two trailer octets sit inside that padded region.
//
// Transport mode: the packet keeps its own IP header outside the ESP
// payload (RFC 4303 Section 3.1.1), so the same subtraction and alignment
// apply to the part after the header, and the header is added back. The
// mode is read from the overhead so tunnel-mode arithmetic is never applied
// to a transport-mode SA (AC-16).
func ceiling(underlay uint16, o *espOverhead) uint16 {
	if underlay <= o.outerHeader+o.fixed {
		return 0
	}
	room := alignDown(underlay-o.outerHeader-o.fixed, o.block)
	if room <= espTrailerOctets {
		return 0
	}
	room -= espTrailerOctets
	if o.mode == ipsecinventory.ModeTransport {
		return room + o.outerHeader
	}
	return room
}

// recommended backs off recommendedMarginOctets from the ceiling and lands on
// the cipher's alignment boundary, the trailer accounted for on both sides.
// It answers errNoUsableMTU when the ceiling is 0 or the backed-off value
// lands below the inner family's minimum: a tunnel that cannot carry the
// minimum packet its family requires has no safe size, and the operator is
// told so rather than handed a small number (AC-6).
func recommended(ceil uint16, o *espOverhead, inner ipFamily) (uint16, error) {
	floor, err := minimumMTU(inner)
	if err != nil {
		return 0, err
	}
	if ceil == 0 {
		return 0, fmt.Errorf("%w: the path leaves no room for a packet after ESP", errNoUsableMTU)
	}
	if ceil+espTrailerOctets < recommendedMarginOctets {
		return 0, fmt.Errorf("%w: a ceiling of %d is below the %d-octet margin", errNoUsableMTU, ceil, recommendedMarginOctets)
	}
	value := alignDown(ceil+espTrailerOctets-recommendedMarginOctets, o.block)
	if value <= espTrailerOctets {
		return 0, fmt.Errorf("%w: a ceiling of %d leaves nothing above the trailer", errNoUsableMTU, ceil)
	}
	value -= espTrailerOctets
	if value < floor {
		return 0, fmt.Errorf("%w: a ceiling of %d leaves %d, below the %s minimum link MTU of %d", errNoUsableMTU, ceil, value, inner, floor)
	}
	return value, nil
}

// minimumMTU is the floor a recommendation must clear for one inner family.
func minimumMTU(inner ipFamily) (uint16, error) {
	switch inner {
	case ipFamilyV4:
		return ipv4MinimumMTU, nil
	case ipFamilyV6:
		return ipv6MinimumLinkMTU, nil
	default:
		return 0, errors.New("mtu: the inner address family is not specified")
	}
}

// mss is the TCP maximum segment size for one tunnel MTU: the MTU less the
// inner IP header and the 20-octet TCP header. It answers errNoUsableMTU when
// the MTU leaves no room for the headers, so an impossible figure is never
// shown as usable.
func mss(mtu uint16, inner ipFamily) (uint16, error) {
	var header uint16
	switch inner {
	case ipFamilyV4:
		header = ipv4HeaderOctets
	case ipFamilyV6:
		header = ipv6HeaderOctets
	default:
		return 0, errors.New("mtu: the inner address family is not specified")
	}
	if mtu <= header+tcpHeaderOctets {
		return 0, fmt.Errorf("%w: an MTU of %d leaves no room for a %s TCP segment", errNoUsableMTU, mtu, inner)
	}
	return mtu - header - tcpHeaderOctets, nil
}
