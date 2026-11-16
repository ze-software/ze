// Design: docs/architecture/wire/nlri-bgpls.md -- LS next-hop framing at public codec entry points.
// Related: docs/architecture/update-building.md -- generic plugin UPDATE builder.
// RFC 9552 Section 5.5 -- see rfc/short/rfc9552.md.
// The LS72 pair oracle follows Wireshark v4.4.0 decode_mp_next_hop_vpn_ipv6:
// https://github.com/wireshark/wireshark/blob/v4.4.0/epan/dissectors/packet-bgp.c
// This decoder-backed layout does not claim another router accepts the pair.

package ls_test

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// TestLSVPNNextHopTypedWire checks literal LS72 RD framing in both codec
// directions, with LS71 singles and pairs as useful plain-address survivors.
// The nonzero NLRI RD must never become the next-hop RD.
func TestLSVPNNextHopTypedWire(t *testing.T) {
	t.Parallel()
	for _, tc := range lsNextHopWireCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			t.Run("encode", func(t *testing.T) {
				mp := attribute.NewMPReachNLRI(attribute.AFI(tc.family.AFI),
					attribute.SAFI(tc.family.SAFI), tc.nextHops, tc.nlri)
				wantLen := 5 + len(tc.wantNextHop) + len(tc.nlri)
				assert.Equal(t, wantLen, mp.Len(), "MP_REACH value length")

				// Poison the destination so each emitted zero, including the RD and
				// reserved octet, must be written by the producer.
				buf := bytes.Repeat([]byte{0xa5}, 4+mp.Len()+3)
				n := mp.WriteTo(buf, 4)
				require.Equal(t, mp.Len(), n, "Len and WriteTo must agree")
				assert.Equal(t, []byte{0xa5, 0xa5, 0xa5, 0xa5}, buf[:4])
				assert.Equal(t, []byte{0xa5, 0xa5, 0xa5}, buf[4+n:])
				checkLSNextHopWire(t, buf[4:4+n], tc)
			})
			t.Run("decode", func(t *testing.T) {
				// Construct the expected wire independently: decoding the encoder's
				// output alone would let two matching framing defects pass together.
				value := make([]byte, 5+len(tc.wantNextHop)+len(tc.nlri))
				value[0], value[1] = 0x40, 0x04
				value[2] = byte(tc.family.SAFI)
				value[3] = byte(len(tc.wantNextHop))
				copy(value[4:], tc.wantNextHop)
				copy(value[5+len(tc.wantNextHop):], tc.nlri)

				back, err := attribute.ParseMPReachNLRI(value)
				require.NoError(t, err, "literal MP_REACH = %x", value)
				assert.Equal(t, attribute.AFI(tc.family.AFI), back.AFI)
				assert.Equal(t, attribute.SAFI(tc.family.SAFI), back.SAFI)
				assert.Equal(t, tc.nextHops, back.NextHops.Slice(), "decoded address slots")
				assert.Equal(t, tc.nlri, back.NLRI, "NLRI, including its own RD, is unchanged")
			})
		})
	}
}

// TestLSVPNPluginBuilderNextHopWire checks that the existing plugin builder
// emits RD-prefixed LS72 singles and keeps the LS71 single-address controls.
// PluginParams has one next hop; paired encoding belongs to the typed codec.
func TestLSVPNPluginBuilderNextHopWire(t *testing.T) {
	t.Parallel()
	for _, tc := range lsNextHopWireCases() {
		if len(tc.nextHops) != 1 {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// A real local AS and ASN4 context avoid an unrelated AS_PATH defect
			// obscuring the MP_REACH producer being tested.
			builder := message.GetUpdateBuilder(65001, false, true, false)
			defer message.PutUpdateBuilder(builder)
			update := builder.BuildPlugin(message.PluginParams{
				AFI:     uint16(tc.family.AFI),
				SAFI:    byte(tc.family.SAFI),
				NLRI:    tc.nlri,
				NextHop: tc.nextHops[0],
			})
			require.NotNil(t, update, "the LS single must produce a useful announcement")
			_, flags, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
			require.True(t, found, "missing MP_REACH_NLRI: %x", update.PathAttributes)
			assert.Equal(t, attribute.FlagOptional, flags)
			checkLSNextHopWire(t, value, tc)
		})
	}
}

type lsNextHopWireCase struct {
	name        string
	family      family.Family
	nextHops    []netip.Addr
	wantNextHop []byte
	nlri        []byte
}

// lsNextHopWireCases supplies independent literal address bytes and valid Node
// NLRIs. LS72 has a type-0 RD (65001:7) inside its NLRI, distinct from the zero
// RD before each next-hop address. Both NLRIs retain the same node descriptors.
func lsNextHopWireCases() []lsNextHopWireCase {
	plainNLRI := []byte{
		0, 1, 0, 37, // Node NLRI, payload length 37.
		3, 0, 0, 0, 0, 0, 0, 1, 0, // OSPFv2, identifier 256.
		1, 0, 0, 24, // Local Node Descriptors, three eight-octet sub-TLVs.
		2, 0, 0, 4, 0, 0, 0xfd, 0xe9, // AS 65001.
		2, 1, 0, 4, 0x12, 0x34, 0x56, 0x78, // BGP-LS identifier.
		2, 3, 0, 4, 192, 0, 2, 9, // IGP Router ID.
	}
	vpnNLRI := []byte{
		0, 1, 0, 45, // Node NLRI, payload length includes its eight-octet RD.
		0, 0, 0xfd, 0xe9, 0, 0, 0, 7, // NLRI RD 65001:7, deliberately nonzero.
		3, 0, 0, 0, 0, 0, 0, 1, 0,
		1, 0, 0, 24,
		2, 0, 0, 4, 0, 0, 0xfd, 0xe9,
		2, 1, 0, 4, 0x12, 0x34, 0x56, 0x78,
		2, 3, 0, 4, 192, 0, 2, 9,
	}
	ipv4 := netip.MustParseAddr("192.0.2.1")
	global := netip.MustParseAddr("2001:db8::1")
	linkLocal := netip.MustParseAddr("fe80::2")
	return []lsNextHopWireCase{
		{
			name: "ls72/ipv4", family: ls.BGPLSVPNFamily, nlri: vpnNLRI,
			nextHops: []netip.Addr{ipv4},
			wantNextHop: []byte{
				0, 0, 0, 0, 0, 0, 0, 0, 192, 0, 2, 1,
			},
		},
		{
			name: "ls72/ipv6", family: ls.BGPLSVPNFamily, nlri: vpnNLRI,
			nextHops: []netip.Addr{global},
			wantNextHop: []byte{
				0, 0, 0, 0, 0, 0, 0, 0,
				0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
			},
		},
		{
			name: "ls72/pair", family: ls.BGPLSVPNFamily, nlri: vpnNLRI,
			nextHops: []netip.Addr{global, linkLocal},
			wantNextHop: []byte{
				0, 0, 0, 0, 0, 0, 0, 0,
				0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
				0, 0, 0, 0, 0, 0, 0, 0,
				0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2,
			},
		},
		{
			name: "ls71/ipv4", family: ls.BGPLSFamily, nlri: plainNLRI,
			nextHops: []netip.Addr{ipv4}, wantNextHop: []byte{192, 0, 2, 1},
		},
		{
			name: "ls71/ipv6", family: ls.BGPLSFamily, nlri: plainNLRI,
			nextHops: []netip.Addr{global},
			wantNextHop: []byte{
				0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
			},
		},
		{
			name: "ls71/pair", family: ls.BGPLSFamily, nlri: plainNLRI,
			nextHops: []netip.Addr{global, linkLocal},
			wantNextHop: []byte{
				0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
				0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2,
			},
		},
	}
}

// checkLSNextHopWire checks the complete field against the literal oracle,
// including every zero RD, address, reserved octet and unchanged NLRI byte.
func checkLSNextHopWire(t *testing.T, value []byte, tc lsNextHopWireCase) {
	t.Helper()
	require.GreaterOrEqual(t, len(value), 5, "short MP_REACH: %x", value)
	assert.Equal(t, []byte{0x40, 0x04, byte(tc.family.SAFI)}, value[:3], "AFI/SAFI")
	require.Equal(t, len(tc.wantNextHop), int(value[3]), "next-hop length; MP_REACH = %x", value)
	require.Len(t, value, 5+len(tc.wantNextHop)+len(tc.nlri), "complete MP_REACH value")
	assert.Equal(t, tc.wantNextHop, value[4:4+len(tc.wantNextHop)], "next-hop bytes")
	assert.Zero(t, value[4+len(tc.wantNextHop)], "reserved octet")
	assert.Equal(t, tc.nlri, value[5+len(tc.wantNextHop):], "unchanged NLRI")
}
