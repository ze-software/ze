// Design: docs/architecture/bgp/structural-forwarding.md -- final next-hop scope on both relays
// Related: docs/architecture/wire/nlri-bgpls.md -- SAFI 71 global and paired next hops
// Related: next_hop_auto_relay_test.go -- receive-to-recording-writer fixtures
// Related: link_scope_fixture_test.go -- shared mode, topology and scoped session helpers
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// LS71 must apply the joint-subnet condition to the effective field, retaining
// both address slots on-link, trimming off-link, and growing a single global
// only with that entity's known LL. Every assertion observes a recording Session
// writer after actual receipt; this is not daemon or external-router evidence.
// RFC 9552 Section 5.5: "The IPv6 link-local address should be used as
// described in [RFC2545]."
// RFC 2545 Section 3: "The link-local address shall be included in the Next Hop
// field if and only if the BGP speaker shares a common subnet with the entity
// identified by the global IPv6 address carried in the Network Address of Next
// Hop field and the peer the route is being advertised to."
func TestLS71PairForwardScope(t *testing.T) {
	for _, mode := range ls71PairModes() {
		for _, native := range []bool{false, true} {
			for _, topology := range ls71PairTopologies() {
				for _, pair := range []bool{false, true} {
					for _, policy := range []bool{false, true} {
						name := mode.name + "/" + nextHopAutoRailName(native) + "/" + topology.name
						if pair {
							name += "/received32"
						} else {
							name += "/received16"
						}
						if policy {
							name += "/policy-base-mixed"
						}
						t.Run(name, func(t *testing.T) {
							f := ls71PairFixture(t, mode, topology)
							field := topology.received.AsSlice()
							if pair {
								field = append(field, netip.MustParseAddr("fe80::9").AsSlice()...)
							}
							body := ls71PairBody(t, field, policy)
							if policy {
								// A raw export answer precedes configured mode selection.
								// Preserving modes keep its field; rewriting modes select
								// their own global and that global's entity LL afterward.
								ls71PairPolicy(t, f, body)
								body = ls71PairBody(t, mustHex(t, "20010db8000100000000000000000007fe800000000000000000000000000007"), true)
							}
							original := bytes.Clone(body)
							rfc4271ProofRelay(t, f, body, native)
							ls71PairAssert(t, f, ls71PairWanted(mode, topology, pair), false, policy)
							require.Equal(t, original, body, "cached received bytes stay borrowed and unchanged")
						})
					}
				}
			}
		}
	}
}

// The literal full Node NLRI has OSPFv2 protocol, identifier 256, and ordered
// AS/BGP-LS-ID/IGP-Router-ID descriptors. Neither NH construction nor decoding
// computes the oracle for these bytes.
func ls71PairNLRI(t *testing.T) []byte {
	t.Helper()
	return mustHex(t, "0001002503000000000000010001000018020000040000fde9020100041234567802030004c0000209")
}

func ls71PairBody(t *testing.T, field []byte, sibling bool) []byte {
	t.Helper()
	value := append([]byte{0x40, 0x04, 71, byte(len(field))}, field...)
	value = append(value, 0)
	value = append(value, ls71PairNLRI(t)...)
	attrs := mixedAttrs(mixedAttr(14, value))
	var legacy []byte
	if sibling {
		attrs = append(attrs, 0x40, 3, 4, 198, 18, 231, 1)
		legacy = []byte{24, 10, 91, 0}
	}
	return makeUpdateBody(nil, attrs, legacy)
}

func ls71PairPolicy(t *testing.T, f *aigpReplayFixture, replacement []byte) {
	t.Helper()
	scope := f.destination.llScope.Load()
	var builtinRef string
	if intent, static := staticExportNextHop(f.destination.settings.ExportPolicy); static {
		switch intent.Mode {
		case policychain.NextHopModeAuto:
			builtinRef = policychain.BuiltinNextHopAuto
		case policychain.NextHopModeSelf:
			builtinRef = policychain.BuiltinNextHopSelf
		case policychain.NextHopModeUnchanged:
			builtinRef = policychain.BuiltinNextHopUnchanged
		case policychain.NextHopModeExplicit, policychain.NextHopModeUnspecified:
			t.Fatalf("unexpected static next-hop mode %v", intent.Mode)
		default:
			t.Fatalf("unexpected static next-hop mode %v", intent.Mode)
		}
	} else {
		// Whole-UPDATE raw overrides terminate the chain early; a preceding
		// next-hop:self with LocalAddress (topology.explicit) exercises the
		// same post-raw next-hop selection onto the explicit global address.
		f.destination.session.transport.Store(&sessionTransport{
			local:       f.destination.settings.LocalAddress,
			localString: f.destination.settings.LocalAddress.String(),
		})
		builtinRef = policychain.BuiltinNextHopSelf
	}
	f.r.orderedEgressSteps = buildOrderedEgressSteps()
	transport := &answerTransport{
		reads: []string{rpc.FilterFieldRaw},
		answer: func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
			return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: replacement}
		},
	}
	f.r.filterTransportSeam = transport
	f.destination.settings.ExportPolicy = exportNextHopPolicy(t, []policychain.Member{
		{Name: builtinRef},
		{Name: exportLibraryFilter, Plugin: answerPlugin},
	})
	f.destination.llScope.Store(scope)
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
	t.Cleanup(func() { require.Positive(t, transport.calls(), "export policy must actually run") })
}

func ls71PairWanted(mode ls71PairMode, topology ls71PairTopology, receivedPair bool) []byte {
	global := topology.self
	ll := netip.MustParseAddr("fe80::1")
	wantPair := topology.joint && topology.ownLL
	if mode.mode == policychain.NextHopModeExplicit {
		global = topology.explicit
	}
	if mode.preserving {
		global = topology.received
		ll = netip.MustParseAddr("fe80::9")
		wantPair = topology.joint && (receivedPair || topology.neighborLL)
	}
	want := global.AsSlice()
	if wantPair {
		want = append(want, ll.AsSlice()...)
	}
	return want
}

// Read every complete UPDATE, requiring the precise LS NLRI and NH bytes, the
// reserved zero, and the independent useful IPv4 sibling when mixed is true.
func ls71PairAssert(t *testing.T, f *aigpReplayFixture, want []byte, withdrawn, mixed bool) {
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
			require.Equal(t, append([]byte{0x40, 0x04, 71}, ls71PairNLRI(t)...), unreach)
			continue
		}
		require.False(t, hasUnreach)
		require.True(t, hasReach)
		value := append([]byte{0x40, 0x04, 71, byte(len(want))}, want...)
		value = append(value, 0)
		value = append(value, ls71PairNLRI(t)...)
		require.Equal(t, value, reach, "exact complete LS71 field and NLRI")
	}
	require.Equal(t, 1, nativeCount, "exactly one LS71 announcement or withdrawal")
	wantSiblings := 0
	if mixed {
		wantSiblings = 1
	}
	require.Equal(t, wantSiblings, siblingCount, "useful sibling survives independently")
}
