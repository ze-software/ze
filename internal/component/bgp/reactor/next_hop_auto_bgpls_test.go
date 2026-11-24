// Design: docs/architecture/bgp/structural-forwarding.md -- family-selected Auto and Self encoding
// Related: docs/architecture/wire/nlri-bgpls.md -- BGP-LS next-hop wire forms
// Related: next_hop_auto_relay_test.go -- shared receive-to-socket relay fixtures
// RFC: rfc/short/rfc9552.md -- Section 5.5, next-hop length selects the address family
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
)

// A usable IPv4 address must leave as four native octets for BGP-LS on either
// relay. A mapped sixteen-octet replacement changes its wire family and causes
// the global-address gate to withdraw an otherwise usable announcement.
func TestNextHopAutoBGPLSNativeIPv4(t *testing.T) {
	tc := nextHopAutoBGPLSFamily()
	for _, direct := range []bool{true, false} {
		for _, native := range []bool{false, true} {
			for _, mode := range []struct {
				name string
				mode policychain.NextHopMode
				hop  netip.Addr
			}{
				{"auto", policychain.NextHopModeAuto, tc.self},
				{"self", policychain.NextHopModeSelf, tc.self},
				{"explicit", policychain.NextHopModeExplicit, tc.explicit},
			} {
				t.Run(nextHopAutoCaseName(tc, direct, native)+"/"+mode.name, func(t *testing.T) {
					f := nextHopAutoFixture(t, tc, direct)
					switch mode.mode {
					case policychain.NextHopModeAuto:
						f.destination.settings.ExportPolicy = setExportNextHopAuto(f.destination.settings.ExportPolicy)
					case policychain.NextHopModeSelf:
						f.destination.settings.ExportPolicy = setExportNextHopSelf(f.destination.settings.ExportPolicy)
					case policychain.NextHopModeExplicit:
						attachExplicitNextHop(t, f.r, f.destination, tc.explicit)
					case policychain.NextHopModeUnchanged, policychain.NextHopModeUnspecified:
						t.Fatalf("unexpected next-hop mode %v", mode.mode)
					default:
						t.Fatalf("unexpected next-hop mode %v", mode.mode)
					}
					f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
					rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
					updates := nextHopAutoUpdates(t, f)
					require.Len(t, updates, 1)
					hop, announced, withdrawn := nextHopAutoFields(t, tc, updates[0])
					require.Equal(t, mode.hop.AsSlice(), hop, "BGP-LS carries a native IPv4 next hop")
					require.Equal(t, tc.prefix, announced)
					require.Empty(t, withdrawn)
				})
			}
		}
	}
}

// The family contract must preserve a received native IPv4 next hop when the
// destination's mode or role requires preservation. Each permitted announcement
// reaches the socket on both relays, including a multihop destination.
func TestNextHopAutoBGPLSPreservesReceivedIPv4(t *testing.T) {
	tc := nextHopAutoBGPLSFamily()
	for _, direct := range []bool{true, false} {
		for _, native := range []bool{false, true} {
			for _, role := range []string{"ibgp", "rs-client", "unchanged"} {
				t.Run(nextHopAutoCaseName(tc, direct, native)+"/"+role, func(t *testing.T) {
					f := nextHopAutoFixture(t, tc, direct)
					f.destination.settings.ExportPolicy = setExportNextHopAuto(f.destination.settings.ExportPolicy)
					if role == "ibgp" {
						f.destination.settings.PeerAS = f.destination.settings.LocalAS
					}
					if role == "rs-client" {
						f.destination.settings.RSClient = true
					}
					if role == "unchanged" {
						f.destination.settings.ExportPolicy = setExportNextHopUnchanged(f.destination.settings.ExportPolicy)
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

// A family-specific native IPv4 rewrite still owes peer-own admission. Seed a
// useful route, replace its next hop with the recipient's address, and require
// an exact withdrawal before corrected Auto can announce the route again.
func TestNextHopAutoBGPLSPeerOwnRecovery(t *testing.T) {
	tc := nextHopAutoBGPLSFamily()
	for _, direct := range []bool{true, false} {
		for _, native := range []bool{false, true} {
			t.Run(nextHopAutoCaseName(tc, direct, native), func(t *testing.T) {
				f := nextHopAutoFixture(t, tc, direct)
				f.destination.settings.ExportPolicy = setExportNextHopAuto(f.destination.settings.ExportPolicy)
				f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
				updates := nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, announced, withdrawn := nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, tc.self.AsSlice(), hop)
				require.Equal(t, tc.prefix, announced)
				require.Empty(t, withdrawn)

				f.conn.reset()
				attachExplicitNextHop(t, f.r, f.destination, f.destination.Settings().Address)
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
				updates = nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, announced, withdrawn = nextHopAutoFields(t, tc, updates[0])
				require.Empty(t, hop)
				require.Empty(t, announced)
				require.Equal(t, tc.prefix, withdrawn)

				f.conn.reset()
				f.destination.settings.ExportPolicy = setExportNextHopAuto(f.destination.settings.ExportPolicy)
				f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
				updates = nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, announced, withdrawn = nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, tc.self.AsSlice(), hop)
				require.Equal(t, tc.prefix, announced)
				require.Empty(t, withdrawn)
			})
		}
	}
}

func nextHopAutoBGPLSFamily() nextHopAutoFamily {
	return nextHopAutoFamily{
		name:     "bgp-ls",
		family:   lsFam,
		self:     netip.MustParseAddr("198.18.232.254"),
		received: netip.MustParseAddr("198.18.231.1"),
		explicit: netip.MustParseAddr("192.0.2.99"),
		prefix:   lsNodeNLRI(65001),
	}
}
