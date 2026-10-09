// Design: docs/architecture/bgp/structural-forwarding.md -- joint next-hop subnet condition.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC2545JointSubnetWriter checks the complete recipient field after real
// Session receipt and both forwarding writers. Connected prefixes model S's
// interfaces; the native fixture separately establishes remote address ownership.
// RFC 2545 Section 3: "The link-local address shall be included in the Next Hop
// field if and only if the BGP speaker shares a common subnet with the entity
// identified by the global IPv6 address carried in the Network Address of Next
// Hop field and the peer the route is being advertised to."
// RFC requirement: RFC2545-3-3 positive -- a received pair on one common subnet reaches the recipient unchanged.
// RFC requirement: RFC2545-3-3 negative -- separate connected subnets or either off-link entity remove the link-local half without withdrawing the NLRI.
// RFC requirement: RFC2545-3-4 positive -- split-link and off-link cases emit exactly the sixteen-byte global field and unchanged NLRI.
// RFC requirement: RFC2545-3-4 negative -- a received pair on one common subnet is not reduced to global-only.
// MUTATION: retain pairs using independent union membership instead of one common
// prefix, bypass either membership condition, or strip every received pair.
func TestRFC2545JointSubnetWriter(t *testing.T) {
	for _, rail := range []string{"cached", "rs"} {
		for _, cap77 := range []bool{false, true} {
			capName := "without-cap77"
			if cap77 {
				capName = "with-cap77"
			}
			for _, tc := range []struct {
				name, peer, global string
				pair               bool
			}{
				{"same-link", "2001:db8:a::2", "2001:db8:a::9", true},
				{"split-link", "2001:db8:a::2", "2001:db8:b::9", false},
				{"entity-off-link", "2001:db8:a::2", "2001:db8:c::9", false},
				{"recipient-off-link", "2001:db8:c::2", "2001:db8:b::9", false},
			} {
				t.Run(rail+"/"+capName+"/"+tc.name, func(t *testing.T) {
					f := rfc2545ReceiveFixture(t, cap77, false, false)
					// Re-key the fixture before dispatch: selectors, pool ownership and
					// immutable forwarding facts must all name the actual IPv6 recipient.
					delete(f.r.peers, f.destination.Settings().PeerKey())
					f.destination.settings.Address = netip.MustParseAddr(tc.peer)
					f.destination.settings.LocalAddress = netip.MustParseAddr("2001:db8:a::1")
					f.r.peers[f.destination.Settings().PeerKey()] = f.destination
					f.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: f.destination.Settings().PeerKey()}, 4096)
					f.destination.refreshLinkScopeFrom([]netip.Prefix{
						netip.MustParsePrefix("2001:db8:a::1/64"),
						netip.MustParsePrefix("2001:db8:b::1/64"),
					})
					f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
					global := netip.MustParseAddr(tc.global).AsSlice()
					pair := append(global[:len(global):len(global)], netip.MustParseAddr("fe80::9").AsSlice()...)
					raw := mustHex(t, "3020010db85701")
					// RFC 2545 Section 3: inspect the effective field at the final writer.
					updates := rfc2545ReceiveForward(t, f, rail, buildUpdatePayload(mixedAttrs(mixedReach(1, pair, raw)), nil))
					if len(updates) != 1 {
						t.Fatalf("recipient UPDATE count=%d, want one announcement", len(updates))
					}
					want := global
					if tc.pair {
						want = pair
					}
					// RFC 2545 Section 3: length, both addresses, reserved octet and NLRI.
					rfc2545AssertNative(t, updates[0], want, raw, true)
					if _, _, _, found := attribute.AttrFind(updates[0].PathAttributes, attribute.AttrMPUnreachNLRI); found {
						t.Error("pair trimming must not withdraw the route")
					}
				})
			}
		}
	}
}
