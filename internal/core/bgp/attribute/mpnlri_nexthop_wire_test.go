package attribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mpReachPoison fills the write buffer before every encode, so an octet the
// writer never touched inside the span Len() promised is visible as 0xAA rather
// than as a zero that reads like a legitimate Reserved octet or a legitimate RD.
const mpReachPoison = 0xAA

// TestMPReachNextHopLengthCountsTheOctetsWritten pins the one arithmetic that
// holds the MP_REACH_NLRI attribute together.
//
// RFC 4760 Section 3 puts a "Length of Next Hop Network Address" octet in front of
// the "Network Address of Next Hop" field, and a receiver takes the Reserved octet
// and the NLRI from the offsets that octet implies. So the number written there,
// the number Len() promises, and the number of octets WriteTo actually emits must
// be one number.
//
// VALIDATES: for every next-hop form the encoder supports, Len() equals WriteTo's
// return, the length octet equals the octets between it and Reserved, Reserved is
// zero, and the NLRI round-trips through ParseMPReachNLRI.
// PREVENTS: the length being derived from the address FAMILY while the bytes come
// from netip.Addr.AsSlice. Those two disagree for exactly one input, the zero
// Addr: it is not Is4, so a family test counted sixteen octets, while AsSlice
// returns none. The attribute then over-stated its next hop by sixteen octets, and
// a receiver read Reserved and the NLRI as part of the next-hop field and took its
// prefixes from whatever the pooled build buffer held after them.
//
// The zero-Addr row is the discriminating one: restore the family test and it
// reports Len 26 against 10 written, a length octet of 16, and an NLRI that is
// buffer poison. Every other row passes under either rule.
//
// RFC requirement: RFC4760-3-2 positive -- the Length of Next Hop Network Address
// field is what tells a receiver the next hop's network-layer protocol, and RFC
// 5549 Section 3 makes reading it a MUST. A length that counts octets the writer
// never emitted names no protocol at all, so this pins the length against the
// bytes for every supported form and re-derives both through ParseMPReachNLRI
// (internal/core/bgp/attribute/mpnlri.go, MPReachNLRI.nextHopOctets and
// MPReachNLRI.WriteTo).
func TestMPReachNextHopLengthCountsTheOctetsWritten(t *testing.T) {
	// 2001:db8::/32 and 10.0.0.0/24 in RFC 4271 Section 4.3 prefix encoding.
	nlri6 := []byte{0x20, 0x20, 0x01, 0x0d, 0xb8}
	nlri4 := []byte{0x18, 0x0a, 0x00, 0x00}

	v4 := netip.MustParseAddr("10.0.0.1")
	v6 := netip.MustParseAddr("2001:db8::1")
	ll := netip.MustParseAddr("fe80::1")

	tests := []struct {
		name      string
		afi       AFI
		safi      SAFI
		nextHops  []netip.Addr
		nlri      []byte
		wantNHLen int // RFC 4760 Section 3: Length of Next Hop Network Address
	}{
		{"ipv4 unicast", AFIIPv4, SAFIUnicast, []netip.Addr{v4}, nlri4, 4},
		{"ipv6 unicast", AFIIPv6, SAFIUnicast, []netip.Addr{v6}, nlri6, 16},
		// RFC 2545 Section 3: global followed by link-local.
		{"ipv6 unicast global and link-local", AFIIPv6, SAFIUnicast, []netip.Addr{v6, ll}, nlri6, 32},
		// RFC 5549 Section 3: an IPv6 next hop for IPv4 NLRI.
		{"ipv4 unicast over an ipv6 next hop", AFIIPv4, SAFIUnicast, []netip.Addr{v6}, nlri4, 16},
		// RFC 4364 Section 4.3.4: an 8-octet zero RD in front of each address.
		{"vpn ipv4", AFIIPv4, SAFIVPN, []netip.Addr{v4}, nlri4, 12},
		{"vpn ipv6", AFIIPv6, SAFIVPN, []netip.Addr{v6}, nlri6, 24},
		// The zero Addr has no wire form. ValidateNextHops refuses it (covered by
		// TestMPReachValidateNextHopsRefusesAnAddressWithNoWireForm); what this row
		// pins is that the length and the write still cannot disagree about it.
		{"no wire form", AFIIPv6, SAFIUnicast, []netip.Addr{{}}, nlri6, 0},
		{"no wire form, vpn", AFIIPv6, SAFIVPN, []netip.Addr{{}}, nlri6, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMPReachNLRI(tt.afi, tt.safi, tt.nextHops, tt.nlri)

			buf := make([]byte, 128)
			for i := range buf {
				buf[i] = mpReachPoison
			}
			n := m.WriteTo(buf, 0)

			// AFI(2) + SAFI(1) + NHLen(1) + NextHop + Reserved(1) + NLRI.
			wantLen := 2 + 1 + 1 + tt.wantNHLen + 1 + len(tt.nlri)
			assert.Equal(t, wantLen, n, "octets written")
			assert.Equal(t, wantLen, m.Len(), "Len() must promise what WriteTo writes")

			require.GreaterOrEqual(t, n, 5)
			assert.Equal(t, byte(tt.wantNHLen), buf[3], "Length of Next Hop Network Address octet")
			assert.Equal(t, byte(0), buf[4+tt.wantNHLen], "RFC 4760 Section 3: Reserved MUST be set to 0")
			assert.Equal(t, tt.nlri, buf[4+tt.wantNHLen+1:n], "NLRI position")
			assert.NotContains(t, buf[:n], byte(mpReachPoison), "an octet inside the declared span was never written")

			// A receiver's own arithmetic, run against the bytes just produced.
			parsed, err := ParseMPReachNLRI(buf[:n])
			require.NoError(t, err)
			assert.Equal(t, tt.afi, parsed.AFI)
			assert.Equal(t, tt.safi, parsed.SAFI)
			assert.Equal(t, tt.nlri, parsed.NLRI, "the NLRI a receiver recovers")
		})
	}
}

// TestMPReachValidateNextHopsRefusesAnAddressWithNoWireForm covers the refusal
// half of the same rule.
//
// A self-consistent encoding of the zero Addr is a next-hop length of zero, which
// ValidNextHopLens admits for no AFI/SAFI pair, so the UPDATE would still be
// malformed at the receiver (RFC 7606 Section 7.11). The attribute is refused
// rather than encoded.
//
// VALIDATES: ValidateNextHops names the zero Addr in either next-hop slot and
// accepts every address that has a wire form; CheckedWriteTo refuses on the same
// answer and writes nothing.
// PREVENTS: an announce rail that reaches the encoder with an unresolved next hop
// putting a zero-length Network Address of Next Hop field on the wire.
//
// RFC requirement: RFC4760-3-2 negative -- a next hop with no wire form maps to no
// network-layer protocol encoding, and ValidNextHopLens admits no length for it,
// so the attribute is refused with ErrUnencodableNextHop rather than encoded with
// a length a receiver cannot resolve to a protocol
// (internal/core/bgp/attribute/mpnlri.go, MPReachNLRI.ValidateNextHops).
func TestMPReachValidateNextHopsRefusesAnAddressWithNoWireForm(t *testing.T) {
	nlri := []byte{0x20, 0x20, 0x01, 0x0d, 0xb8}
	v6 := netip.MustParseAddr("2001:db8::1")
	ll := netip.MustParseAddr("fe80::1")

	tests := []struct {
		name     string
		nextHops []netip.Addr
		wantErr  bool
	}{
		{"global address", []netip.Addr{v6}, false},
		{"global and link-local", []netip.Addr{v6, ll}, false},
		{"ipv4 address", []netip.Addr{netip.MustParseAddr("10.0.0.1")}, false},
		{"zero addr alone", []netip.Addr{{}}, true},
		{"zero addr in the first slot", []netip.Addr{{}, ll}, true},
		{"zero addr in the second slot", []netip.Addr{v6, {}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMPReachNLRI(AFIIPv6, SAFIUnicast, tt.nextHops, nlri)

			err := m.ValidateNextHops()
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrUnencodableNextHop)

			buf := make([]byte, 128)
			for i := range buf {
				buf[i] = mpReachPoison
			}
			n, writeErr := m.CheckedWriteTo(buf, 0)
			require.ErrorIs(t, writeErr, ErrUnencodableNextHop)
			assert.Zero(t, n)
			assert.Equal(t, byte(mpReachPoison), buf[0], "CheckedWriteTo must write nothing when it refuses")
		})
	}
}
