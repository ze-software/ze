// Design: docs/architecture/bgp/structural-forwarding.md -- forwarded next-hop self
// Related: peer_forward_facts.go -- precomputeNextHop and applyFactsNextHop
// Related: relay_proof_helpers_test.go -- receive, both relays and socket barrier
// RFC: rfc/short/rfc4271.md -- Section 5.1.3, default eBGP next hop

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// The omitted leaf is parsed, rather than assuming which enum value is the
// default. RFC 4271 Section 5.1.3 items 2 and 3 prescribe the default interface
// address. The owner chose that default for auto on 2026-10-03 and 2026-11-13.
// VALIDATES: next-hop-auto AC-1..3, AC-8: auto and self emit the same actual
// connected endpoint, ahead of a different configured fallback, on both rails.
func TestNextHopAutoMatchesSelfForEBGP(t *testing.T) {
	for _, tc := range nextHopAutoFamilies() {
		for _, direct := range []bool{true, false} {
			for _, native := range []bool{false, true} {
				t.Run(nextHopAutoCaseName(tc, direct, native), func(t *testing.T) {
					parsed, err := parsePeerFromTree("default", peerTreeWith(tc.self.String(), ""), 65000, 0x01020301)
					require.NoError(t, err)
					require.Equal(t, NextHopAuto, parsed.NextHopMode, "omitted next-hop selects auto")
					for _, mode := range []uint8{parsed.NextHopMode, NextHopSelf} {
						for _, connected := range []bool{false, true} {
							f := nextHopAutoFixture(t, tc, direct)
							f.destination.settings.NextHopMode = mode
							if connected {
								f.destination.settings.LocalAddress = netip.MustParseAddr("192.0.2.254")
							} else {
								f.destination.session.transport.Store(nil)
							}
							f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
							rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
							updates := nextHopAutoUpdates(t, f)
							require.Len(t, updates, 1, "mode=%d connected=%v", mode, connected)
							hop, announced, withdrawn := nextHopAutoFields(t, tc, updates[0])
							require.Equal(t, tc.self.AsSlice(), hop, "mode=%d connected=%v resolves self", mode, connected)
							require.Equal(t, tc.prefix, announced)
							require.Empty(t, withdrawn)
						}
					}
				})
			}
		}
	}
}

// RFC 4271 Section 5.1.3 item 1: a received route toward iBGP SHOULD NOT be
// rewritten without explicit self. RFC 7947 Section 2.2.1 requires preserving
// NEXT_HOP toward an RS client, which the owner's auto behavior table retains.
// VALIDATES: next-hop-auto AC-5..6: neither a blanket auto-to-self change nor
// selecting the native relay as a substitute for the RS-client fact can pass.
func TestNextHopAutoPreservesRSClientAndIBGP(t *testing.T) {
	for _, tc := range nextHopAutoFamilies() {
		for _, role := range []string{"rs-client", "ibgp"} {
			for _, native := range []bool{false, true} {
				t.Run(tc.name+"/"+role+"/"+nextHopAutoRailName(native), func(t *testing.T) {
					f := nextHopAutoFixture(t, tc, true)
					f.destination.settings.NextHopMode = NextHopAuto
					if role == "rs-client" {
						f.destination.settings.RSClient = true
					} else {
						f.destination.settings.PeerAS = f.destination.settings.LocalAS
					}
					negotiateForwardTestFamilies(f.destination, tc.family)
					f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
					rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
					updates := nextHopAutoUpdates(t, f)
					require.Len(t, updates, 1)
					hop, announced, withdrawn := nextHopAutoFields(t, tc, updates[0])
					require.Equal(t, tc.received.AsSlice(), hop)
					require.Equal(t, tc.prefix, announced)
					require.Empty(t, withdrawn)
				})
			}
		}
	}
}

// VALIDATES: next-hop-auto AC-8: the existing explicit modes retain their
// meaning. A fix must distinguish auto from unchanged rather than rewriting
// both, or making every mode choose self.
func TestNextHopAutoChangePreservesExplicitModes(t *testing.T) {
	for _, tc := range nextHopAutoFamilies() {
		for _, native := range []bool{false, true} {
			for _, mode := range []uint8{NextHopUnchanged, NextHopSelf, NextHopExplicit} {
				t.Run(tc.name+"/"+nextHopAutoRailName(native)+"/"+nextHopAutoModeName(mode), func(t *testing.T) {
					f := nextHopAutoFixture(t, tc, true)
					f.destination.settings.NextHopMode = mode
					f.destination.settings.NextHopAddress = tc.explicit
					f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
					rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
					updates := nextHopAutoUpdates(t, f)
					require.Len(t, updates, 1)
					hop, announced, withdrawn := nextHopAutoFields(t, tc, updates[0])
					want := tc.received
					if mode == NextHopSelf {
						want = tc.self
					}
					if mode == NextHopExplicit {
						want = tc.explicit
					}
					require.Equal(t, want.AsSlice(), hop)
					require.Equal(t, tc.prefix, announced)
					require.Empty(t, withdrawn)
				})
			}
		}
	}
}

// VALIDATES: next-hop-auto AC-4: after both routes really reached the socket,
// losing the self address withdraws the would-be announcement and still sends
// a sibling's explicit withdrawal. Rejecting the whole mixed UPDATE fails.
func TestNextHopAutoWithoutOwnAddressPreservesMixedWithdrawals(t *testing.T) {
	for _, tc := range nextHopAutoFamilies() {
		for _, native := range []bool{false, true} {
			t.Run(tc.name+"/"+nextHopAutoRailName(native), func(t *testing.T) {
				f := nextHopAutoFixture(t, tc, true)
				seed := append(append([]byte(nil), tc.prefix...), tc.sibling...)
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, seed, nil), native)
				initial := nextHopAutoUpdates(t, f)
				require.Len(t, initial, 1)
				_, announced, withdrawn := nextHopAutoFields(t, tc, initial[0])
				require.Equal(t, seed, announced)
				require.Empty(t, withdrawn)
				f.conn.reset()
				f.destination.settings.NextHopMode = NextHopAuto
				f.destination.settings.LocalAddress = netip.Addr{}
				f.destination.session.transport.Store(nil)
				f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, tc.sibling), native)
				var got [][]byte
				for _, update := range nextHopAutoUpdates(t, f) {
					hop, reach, unreach := nextHopAutoFields(t, tc, update)
					require.Empty(t, reach, "no fallback to the received third-party next hop")
					require.Empty(t, hop)
					parts, err := nlrisplit.SplitWithdrawn(tc.family, unreach, false)
					require.NoError(t, err)
					got = append(got, parts...)
				}
				require.ElementsMatch(t, [][]byte{tc.prefix, tc.sibling}, got, "both prior routes must be withdrawn exactly once")
			})
		}
	}
}

type nextHopAutoFamily struct {
	name                     string
	family                   family.Family
	self, received, explicit netip.Addr
	prefix, sibling          []byte
}

func nextHopAutoFamilies() []nextHopAutoFamily {
	return []nextHopAutoFamily{
		{"ipv4", family.IPv4Unicast, netip.MustParseAddr("198.18.232.254"), netip.MustParseAddr("198.18.231.1"), netip.MustParseAddr("192.0.2.99"), []byte{24, 10, 88, 0}, []byte{24, 10, 89, 0}},
		{"ipv6", family.IPv6Unicast, netip.MustParseAddr("2001:db8:2::254"), netip.MustParseAddr("2001:db8:11::1"), netip.MustParseAddr("2001:db8:99::1"), []byte{64, 0x20, 1, 0x0d, 0xb8, 0, 0x58, 0, 0}, []byte{64, 0x20, 1, 0x0d, 0xb8, 0, 0x59, 0, 0}},
	}
}

func nextHopAutoFixture(t *testing.T, tc nextHopAutoFamily, direct bool) *aigpReplayFixture {
	t.Helper()
	f := newAIGPReplayFixture(t, nil)
	if tc.family == family.IPv6Unicast {
		rfc2545ForwardScope(f, direct, false)
	}
	f.destination.settings.PeerAS = 65002
	f.destination.settings.LocalAddress = tc.self
	f.destination.settings.LinkLocal = netip.Addr{}
	f.destination.settings.OutTTL = 1
	if !direct {
		f.destination.settings.OutTTL = 3
	}
	f.destination.session.transport.Store(&sessionTransport{local: tc.self, localString: tc.self.String()})
	for _, peer := range []*Peer{f.source, f.destination} {
		negotiateForwardTestFamilies(peer, tc.family)
	}
	connected := []netip.Prefix{netip.PrefixFrom(tc.self, tc.self.BitLen())}
	if direct {
		bits := 24
		if tc.self.Is6() {
			bits = 64
		}
		connected = []netip.Prefix{netip.PrefixFrom(tc.self, bits)}
	}
	f.destination.refreshLinkScopeFrom(connected)
	// refreshForwardFacts re-reads host interfaces and replaces the explicit
	// snapshot. Build over the fixture's connected prefixes instead.
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
	require.Equal(t, direct, destOnLink(f.destination), "fixture must distinguish on-link and multihop recipients")
	return f
}

func nextHopAutoBody(t *testing.T, tc nextHopAutoFamily, announce, withdraw []byte) []byte {
	t.Helper()
	if tc.family == family.IPv4Unicast {
		body := rfc4271ProofRelayBody(announce, false, 10)
		update, err := message.UnpackUpdate(body)
		require.NoError(t, err)
		return makeUpdateBody(withdraw, update.PathAttributes, announce)
	}
	var mp [128]byte
	n := writeMPReach(mp[:], 0, tc.family, tc.received.AsSlice(), announce)
	attrs := mixedAttrs(mp[:n])
	if len(withdraw) != 0 {
		attrs = append(attrs, mixedAttr(byte(attribute.AttrMPUnreachNLRI), mixedUnreachValue(uint16(tc.family.AFI), byte(tc.family.SAFI), withdraw))...)
	}
	return makeUpdateBody(nil, attrs, nil)
}

func nextHopAutoUpdates(t *testing.T, f *aigpReplayFixture) []*message.Update {
	t.Helper()
	var updates []*message.Update
	for _, body := range aigpSocketBodies(t, f.conn) {
		update, err := message.UnpackUpdate(body)
		require.NoError(t, err)
		updates = append(updates, update)
	}
	return updates
}

func nextHopAutoFields(t *testing.T, tc nextHopAutoFamily, update *message.Update) (hop, announce, withdraw []byte) {
	t.Helper()
	if tc.family == family.IPv4Unicast {
		_, _, hop, _ = attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
		return hop, update.NLRI, update.WithdrawnRoutes
	}
	require.Empty(t, update.NLRI)
	require.Empty(t, update.WithdrawnRoutes)
	if _, _, reach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI); found {
		require.GreaterOrEqual(t, len(reach), 5)
		require.Equal(t, uint16(tc.family.AFI), binary.BigEndian.Uint16(reach[:2]))
		require.Equal(t, byte(tc.family.SAFI), reach[2])
		length := int(reach[3])
		require.GreaterOrEqual(t, len(reach), 5+length)
		hop, announce = reach[4:4+length], reach[5+length:]
	}
	if _, _, unreach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI); found {
		require.GreaterOrEqual(t, len(unreach), 3)
		require.Equal(t, uint16(tc.family.AFI), binary.BigEndian.Uint16(unreach[:2]))
		require.Equal(t, byte(tc.family.SAFI), unreach[2])
		withdraw = unreach[3:]
	}
	return hop, announce, withdraw
}

func nextHopAutoModeName(mode uint8) string {
	switch mode {
	case NextHopUnchanged:
		return "unchanged"
	case NextHopSelf:
		return "self"
	default:
		return "explicit"
	}
}

func nextHopAutoCaseName(tc nextHopAutoFamily, direct, native bool) string {
	hops := "multihop"
	if direct {
		hops = "single-hop"
	}
	return tc.name + "/" + hops + "/" + nextHopAutoRailName(native)
}
