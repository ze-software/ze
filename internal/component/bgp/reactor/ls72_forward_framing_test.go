// Design: docs/architecture/bgp/structural-forwarding.md -- effective LS-VPN framing
// Related: docs/architecture/wire/nlri-bgpls.md -- per-address zero RDs and separate NLRI RD
// Related: ls71_pair_scope_test.go -- real receive/cache and recording Session fixtures
// RFC 9552 Section 5.5 -- see rfc/short/rfc9552.md
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// Every configured mode must select a correctly framed useful single next hop
// after actual receipt, on both forwarding rails and both transport distances.
// The preserving twins retain the exact received field. Negotiation includes
// only ASN4 and MP families, so native LS IPv6 cannot require capability 5.
func TestNextHopAutoLSVPNFraming(t *testing.T) {
	for _, ipv6 := range []bool{false, true} {
		for _, direct := range []bool{true, false} {
			for _, native := range []bool{false, true} {
				for _, mode := range ls71PairModes() {
					topology := ls72SingleTopology(ipv6, direct)
					name := "ipv4/"
					if ipv6 {
						name = "ipv6/"
					}
					name += mode.name + "/" + nextHopAutoRailName(native) + "/multihop"
					if direct {
						name = name[:len(name)-len("multihop")] + "direct"
					}
					t.Run(name, func(t *testing.T) {
						f := ls72Fixture(t, mode, topology)
						require.Equal(t, direct, destOnLink(f.destination))
						f.destination.settings.OutTTL = 1
						if !direct {
							f.destination.settings.OutTTL = 3
						}
						f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
						body := ls72Body(t, ls72Framed(topology.received.AsSlice(), nil), false)
						original := bytes.Clone(body)
						ls71PairRelay(t, f, body, native, false)
						global := topology.self
						if mode.mode == NextHopExplicit {
							global = topology.explicit
						}
						if mode.preserving {
							global = topology.received
						}
						ls72Assert(t, f, ls72Framed(global.AsSlice(), nil), false, false)
						require.Equal(t, original, body)
					})
				}
			}
		}
	}
}

func ls72SingleTopology(ipv6, direct bool) ls71PairTopology {
	topology := ls71PairTopologies()[5]
	if !ipv6 {
		topology.self = netip.MustParseAddr("198.18.232.254")
		topology.explicit = netip.MustParseAddr("198.18.232.253")
		topology.received = netip.MustParseAddr("198.18.232.9")
		topology.recipient = netip.MustParseAddr("198.18.232.2")
		topology.connected = []netip.Prefix{netip.MustParsePrefix("198.18.232.0/24")}
	}
	if !direct {
		topology.joint = false
		topology.connected = []netip.Prefix{netip.PrefixFrom(topology.self, topology.self.BitLen())}
	}
	return topology
}

// The owning LS plugin supplies the profile; the reactor gets no synthetic
// registry entry. Reuse the established scope, neighbor, transport and sink
// setup, then negotiate the actual LS72 family on both session endpoints.
func ls72Fixture(t *testing.T, mode ls71PairMode, topology ls71PairTopology) *aigpReplayFixture {
	t.Helper()
	require.Equal(t, []int{12, 24, 48}, attribute.ValidNextHopLens(16388, 72))
	f := ls71PairFixture(t, mode, topology)
	for _, peer := range []*Peer{f.source, f.destination} {
		negotiateForwardTestFamilies(peer, ls.BGPLSVPNFamily, family.IPv4Unicast)
	}
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
	return f
}

// The independently specified Node NLRI includes RD 65001:7 in its 45-octet
// payload. It is deliberately different from every zero next-hop RD.
func ls72NLRI(t *testing.T) []byte {
	t.Helper()
	return mustHex(t, "0001002d0000fde90000000703000000000000010001000018020000040000fde9020100041234567802030004c0000209")
}

// This literal test layout never calls the profile codec or a production NH
// writer: every supplied address has its own eight zero RD octets.
func ls72Framed(primary, secondary []byte) []byte {
	field := append([]byte{0, 0, 0, 0, 0, 0, 0, 0}, primary...)
	if len(secondary) != 0 {
		field = append(field, 0, 0, 0, 0, 0, 0, 0, 0)
		field = append(field, secondary...)
	}
	return field
}

func ls72Body(t *testing.T, field []byte, sibling bool) []byte {
	t.Helper()
	value := append([]byte{0x40, 0x04, 72, byte(len(field))}, field...)
	value = append(value, 0)
	value = append(value, ls72NLRI(t)...)
	attrs := mixedAttrs(mixedAttr(14, value))
	var legacy []byte
	if sibling {
		attrs = append(attrs, 0x40, 3, 4, 198, 18, 231, 1)
		legacy = []byte{24, 10, 91, 0}
	}
	return makeUpdateBody(nil, attrs, legacy)
}

func ls72Received(topology ls71PairTopology, pair bool) []byte {
	var secondary []byte
	if pair {
		secondary = []byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9}
	}
	return ls72Framed(topology.received.AsSlice(), secondary)
}

func ls72Wanted(mode ls71PairMode, topology ls71PairTopology, receivedPair bool) []byte {
	plain := ls71PairWanted(mode, topology, receivedPair)
	if len(plain) == 16 {
		return ls72Framed(plain, nil)
	}
	return ls72Framed(plain[:16], plain[16:])
}

// Check every complete writer result, including the reserved octet, both NH
// RDs, exact nonzero NLRI RD and useful independent classic IPv4 section.
func ls72Assert(t *testing.T, f *aigpReplayFixture, want []byte, withdrawn, mixed bool) {
	t.Helper()
	nativeCount, siblingCount := 0, 0
	for _, update := range nextHopAutoUpdates(t, f) {
		if len(update.NLRI) != 0 {
			siblingCount++
			require.True(t, mixed)
			require.Equal(t, []byte{24, 10, 91, 0}, update.NLRI)
			require.Empty(t, update.WithdrawnRoutes)
			_, _, hop, present := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
			require.True(t, present)
			require.Equal(t, []byte{198, 18, 231, 1}, hop)
			_, _, _, reach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
			_, _, _, unreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
			require.False(t, reach)
			require.False(t, unreach)
			continue
		}
		nativeCount++
		require.Empty(t, update.WithdrawnRoutes)
		_, _, reach, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
		_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
		if withdrawn {
			require.False(t, hasReach)
			require.True(t, hasUnreach)
			require.Equal(t, append([]byte{0x40, 0x04, 72}, ls72NLRI(t)...), unreach)
			continue
		}
		require.False(t, hasUnreach)
		require.True(t, hasReach)
		value := append([]byte{0x40, 0x04, 72, byte(len(want))}, want...)
		value = append(value, 0)
		value = append(value, ls72NLRI(t)...)
		require.Equal(t, value, reach, "exact complete LS72 field and independent NLRI RD")
	}
	require.Equal(t, 1, nativeCount, "exactly one LS72 announcement or withdrawal")
	wantSiblings := 0
	if mixed {
		wantSiblings = 1
	}
	require.Equal(t, wantSiblings, siblingCount, "useful sibling survives independently")
}
