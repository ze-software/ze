// Design: docs/architecture/bgp/structural-forwarding.md -- final next-hop fields and identity
// Related: forward_next_hop.go -- effectiveNextHop and scope normalization
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// Each gate must inspect the fields that survive in its destination section:
// an unused operation cannot invent a sibling, and one replacement cannot erase
// the identity of an untouched sibling. All families here are built-in.
func TestEffectiveNextHopFields(t *testing.T) {
	legacy := netip.MustParseAddr("192.0.2.2")
	global := netip.MustParseAddr("2001:db8::9")
	other := netip.MustParseAddr("2001:db8::7")
	for _, tc := range []struct {
		name             string
		base             nextHopValue
		peer             netip.Addr
		legacySet, mpSet netip.Addr
		own              bool
	}{
		{name: "unused-mp-beside-legacy", base: nextHopValue{legacy: legacy}, peer: global, legacySet: legacy, mpSet: global},
		{name: "unused-legacy-beside-mp", base: nextHopValue{mp: global, mpFamily: family.IPv6Unicast, mpIPv6: true, mpWireLen: 16}, peer: legacy, legacySet: legacy, mpSet: other},
		{name: "untouched-peer-own-legacy", base: nextHopValue{legacy: legacy, mp: global, mpFamily: family.IPv6Unicast, mpIPv6: true, mpWireLen: 16}, peer: legacy, mpSet: other, own: true},
		{name: "untouched-peer-own-mp", base: nextHopValue{legacy: legacy, mp: global, mpFamily: family.IPv6Unicast, mpIPv6: true, mpWireLen: 16}, peer: global, legacySet: legacy, own: true},
		{name: "selected-mp-replaces-obsolete-identity", base: nextHopValue{mp: global, mpFamily: family.IPv6Unicast, mpIPv6: true, mpWireLen: 16}, peer: global, mpSet: other},
		{name: "legacy-classifier-without-source", peer: legacy, legacySet: legacy, own: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mods filterapi.ModAccumulator
			if tc.legacySet.IsValid() {
				mods.Op(uint8(attribute.AttrNextHop), filterapi.AttrModSet, tc.legacySet.AsSlice())
			}
			if tc.mpSet.IsValid() {
				mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, tc.mpSet.AsSlice())
			}
			require.Equal(t, tc.own, egressNextHopIsPeerOwn(&peerForwardFacts{addr: tc.peer}, &mods, tc.base))
		})
	}
}

// The actual destination materializer must agree with the gate's field set:
// MP cannot be invented beside classic NLRI, and code 3 carries no route in a
// native-only section even when a generic attribute operation writes it.
func TestEffectiveNextHopUnusedFieldMaterializer(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		native, unusedLegacy bool
	}{
		{name: "legacy-only"},
		{name: "mp-only", native: true},
		{name: "mp-only-extraneous-legacy", native: true, unusedLegacy: true},
	} {
		peer := netip.MustParseAddr("2001:db8::2")
		body := buildUpdatePayload(mixedAttrs([]byte{0x40, 3, 4, 192, 0, 2, 9}), mixedV4Legacy)
		if tc.native {
			peer = netip.MustParseAddr("192.0.2.2")
			attrs := mixedAttrs(mixedReach(1, netip.MustParseAddr("2001:db8::9").AsSlice(), mixedV6Prefix))
			if tc.unusedLegacy {
				attrs = append(attrs, 0x40, 3, 4, 192, 0, 2, 2)
			}
			body = buildUpdatePayload(attrs, nil)
		}
		t.Run(tc.name, func(t *testing.T) {
			var mods filterapi.ModAccumulator
			legacy := []byte{192, 0, 2, 1}
			mp := netip.MustParseAddr("2001:db8::2").AsSlice()
			if tc.native {
				legacy = []byte{192, 0, 2, 2}
				mp = netip.MustParseAddr("2001:db8::7").AsSlice()
			}
			mods.Op(uint8(attribute.AttrNextHop), filterapi.AttrModSet, legacy)
			mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, mp)
			require.False(t, egressNextHopIsPeerOwn(&peerForwardFacts{addr: peer}, &mods, payloadNextHop(body)))
			output, _, failure := buildModifiedPayload(body, &mods, attrModHandlersWithDefaults(), nil, nil)
			require.Equal(t, modifyFailureNone, failure)
			require.NotNil(t, output)
			update, err := message.UnpackUpdate(output)
			require.NoError(t, err)
			require.Empty(t, update.WithdrawnRoutes)
			_, _, unreach, withdrawn := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
			require.False(t, withdrawn, "unexpected MP withdrawal %x", unreach)
			_, _, reach, present := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
			if tc.native {
				require.Empty(t, update.NLRI)
				require.True(t, present)
				want := append([]byte{0, 2, 1, 16}, mp...)
				want = append(want, 0)
				want = append(want, mixedV6Prefix...)
				require.Equal(t, want, reach)
			} else {
				require.Equal(t, mixedV4Legacy, update.NLRI)
				require.False(t, present, "an operation cannot create MP NLRI")
				_, _, hop, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
				require.True(t, found)
				require.Equal(t, legacy, hop)
			}
			require.False(t, aigpNextHop(output).has(peer), "only fields governing actual NLRI identify its next hop")
		})
	}
}

// Redirect targets use the last effective MP field independently of a useful
// legacy sibling. An empty or unsupported field strips the old target.
func TestEffectiveRedirectNextHopIdentity(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.2")
	for _, mixed := range []bool{false, true} {
		for _, tc := range []struct {
			name string
			sets [][]byte
			own  bool
		}{
			{name: "received", own: true},
			{name: "replaced", sets: [][]byte{{192, 0, 2, 9}}},
			{name: "selected-own", sets: [][]byte{{192, 0, 2, 9}, {192, 0, 2, 2}}, own: true},
			{name: "empty", sets: [][]byte{nil}},
			{name: "unsupported-width", sets: [][]byte{make([]byte, 12)}},
		} {
			name := "native-only/" + tc.name
			if mixed {
				name = "mixed/" + tc.name
			}
			t.Run(name, func(t *testing.T) {
				base := nextHopValue{
					mpFamily:  family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec},
					mpIgnored: true, mpRedirect: true, mpRedirectAddr: peer,
				}
				if mixed {
					base.legacy = netip.MustParseAddr("192.0.2.9")
				}
				var mods filterapi.ModAccumulator
				for _, set := range tc.sets {
					mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, set)
				}
				facts := &peerForwardFacts{addr: peer}
				dest := NewPeer(&PeerSettings{Address: peer})
				require.Equal(t, tc.own, egressNextHopIsPeerOwn(facts, &mods, base))
				want := withholdNone
				if tc.own {
					want = withholdPeerOwn
				}
				require.Equal(t, want, egressNextHopWithheld(dest, facts, &mods, base, false, netip.Addr{}))
			})
		}
	}
}

// A legacy-only edit must leave the received MP link-local field visible to
// reflection, off-link and capability gates; the MP operation cannot affect a
// legacy-only section in the opposite direction.
func TestEffectiveNextHopLinkLocalGates(t *testing.T) {
	var mods filterapi.ModAccumulator
	mods.Op(uint8(attribute.AttrNextHop), filterapi.AttrModSet, []byte{192, 0, 2, 1})
	base := nextHopValue{
		legacy: netip.MustParseAddr("192.0.2.9"), mp: netip.MustParseAddr("fe80::9"),
		mpFamily: family.IPv6Unicast, mpIPv6: true, mpWireLen: 16,
	}
	dest := NewPeer(&PeerSettings{Address: netip.MustParseAddr("2001:db8::2")})
	require.True(t, egressNextHopIsLinkLocalOnly(&mods, base))
	require.True(t, egressNextHopLinkLocalOnlyOffLink(dest, &mods, base))
	require.True(t, egressNextHopLinkLocalOnlyRefused(dest, &mods, base))

	mods.Reset()
	mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, netip.MustParseAddr("fe80::9").AsSlice())
	base = nextHopValue{legacy: netip.MustParseAddr("192.0.2.9")}
	require.False(t, egressNextHopIsLinkLocalOnly(&mods, base))
	require.False(t, egressNextHopLinkLocalOnlyOffLink(dest, &mods, base))
	require.False(t, egressNextHopLinkLocalOnlyRefused(dest, &mods, base))
}

// Scope normalization must retain a selected peer-own pair for the existing
// withdrawal gate before trimming. Plain32 and RD48 use a zoned LL recipient;
// the useful twin and a selected replacement prove obsolete input is harmless.
func TestEffectiveNextHopScopePeerOwn(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fam    family.Family
		prefix []byte
	}{
		{name: "plain", fam: family.IPv6Unicast},
		{name: "rd", fam: family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}, prefix: make([]byte, 8)},
	} {
		for _, rewritten := range []bool{false, true} {
			name := tc.name + "/received"
			if rewritten {
				name = tc.name + "/selected"
			}
			t.Run(name, func(t *testing.T) {
				peer := netip.MustParseAddr("fe80::2%if0")
				dest := NewPeer(&PeerSettings{Address: peer})
				facts := &peerForwardFacts{addr: peer}
				global := netip.MustParseAddr("2001:db8::9").AsSlice()
				field := append(bytes.Clone(tc.prefix), global...)
				globalOctets := len(field)
				field = append(field, tc.prefix...)
				field = append(field, netip.MustParseAddr("fe80::2").AsSlice()...)
				body := buildUpdatePayload(mixedAttrs(mixedReach(byte(tc.fam.SAFI), field, []byte{0})), nil)
				original := bytes.Clone(body)
				var mods filterapi.ModAccumulator
				if rewritten {
					body = buildUpdatePayload(mixedAttrs(mixedReach(byte(tc.fam.SAFI), field[:globalOctets], []byte{0})), nil)
					original = bytes.Clone(body)
					mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, field)
				}
				base := payloadNextHop(body)
				applyEgressNextHopScope(dest, facts, &mods, body, tc.fam)
				require.Equal(t, withholdPeerOwn, egressNextHopWithheld(dest, facts, &mods, base, false, netip.Addr{}))
				require.Equal(t, original, body, "borrowed input stays unchanged")
				require.Equal(t, "if0", dest.settings.Address.Zone())

				// The same off-link pair with another router's LL trims usefully.
				good := bytes.Clone(field)
				good[len(good)-1] = 3
				mods.Reset()
				mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, good)
				applyEgressNextHopScope(dest, facts, &mods, body, tc.fam)
				require.Equal(t, withholdNone, egressNextHopWithheld(dest, facts, &mods, base, false, netip.Addr{}))
				written, set := modsNextHop(&mods, tc.fam)
				require.True(t, set)
				require.Equal(t, uint8(globalOctets), written.mpWireLen)

				// A legitimate self replacement must supersede the bad source.
				mods.Reset()
				mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, field[:globalOctets])
				applyEgressNextHopScope(dest, facts, &mods, body, tc.fam)
				require.Equal(t, withholdNone, egressNextHopWithheld(dest, facts, &mods, base, false, netip.Addr{}))
			})
		}
	}
}
