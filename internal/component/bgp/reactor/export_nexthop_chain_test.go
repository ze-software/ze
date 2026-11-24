// Design: docs/architecture/core-design.md -- Policy Filter Chain, unified export next-hop rewriting
// Related: peer_forward_facts.go -- precomputeNextHop, precomputeRedirectNextHop, redirectForIntent
// Related: forward_next_hop.go -- applyEgressNextHopIntent, applyRawOverrideNextHopIntent, egressNextHopWithheldWithSelf
// Related: egress_inject_filter.go -- exportFilterFromFacts, originatedIntentUsesSelf
// Related: policy_dryrun.go -- PolicyDryRun tracing built-in next-hop steps
// RFC: rfc/short/rfc4271.md -- Section 5.1.3, default eBGP NEXT_HOP rewrite

package reactor

import (
	"net/netip"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	exportNHAttachment = "bgp export"
	exportNHField      = "next-hop"
	exportNHFilter     = "mod"
	exportNHFilter2    = "mod2"
)

// exportNextHopPolicy builds a compiled export filterapi.Policy at "bgp export"
// from members with final accept.
func exportNextHopPolicy(t testing.TB, members []policychain.Member) filterapi.Policy {
	t.Helper()
	chain, err := policychain.Compile(exportNHAttachment, nil, members, routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	require.NoError(t, err)
	refs := make([]routepolicy.Ref, len(members))
	for i, m := range members {
		if m.Plugin == "" {
			refs[i] = routepolicy.Ref{Name: m.Name}
		} else {
			refs[i] = routepolicy.Ref{Name: m.Plugin + ":" + m.Name}
		}
	}
	return mustPolicy(t, refs, routepolicy.FinalAccept).WithLibrary(chain)
}

// attachExportNextHopTransport binds a dynamic export chain on f.destination
// backed by an answerTransport declaring reads and sets on answerPlugin.
func attachExportNextHopTransport(
	t testing.TB,
	f *aigpReplayFixture,
	members []policychain.Member,
	reads, sets []string,
	answer func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput,
) {
	t.Helper()
	f.r.orderedEgressSteps = buildOrderedEgressSteps()
	f.r.filterTransportSeam = &answerTransport{
		reads:  reads,
		sets:   sets,
		answer: answer,
	}
	f.destination.settings.ExportPolicy = exportNextHopPolicy(t, members)
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
}

// TestExportNextHopChainStaticFastPath proves static-only built-in next-hop
// export policies stay off the per-route chain path while non-static,
// templated, and ingress-requiring policies activate it.
//
// VALIDATES: AC-18 zero-allocation static fast path for peer-wide next-hop:*.
// PREVENTS: trailing next-hop:auto forcing every unconfigured peer onto per-route chain evaluation.
func TestExportNextHopChainStaticFastPath(t *testing.T) {
	empty := filterapi.Policy{}
	intent, static := staticExportNextHop(empty)
	require.True(t, static)
	require.Equal(t, policychain.NextHopModeAuto, intent.Mode)
	require.True(t, intent.AutoDefault)
	require.False(t, hasActiveFilter(empty, directionExport))

	for _, tc := range []struct {
		name string
		ref  string
		want policychain.NextHopMode
	}{
		{"auto", policychain.BuiltinNextHopAuto, policychain.NextHopModeAuto},
		{"self", policychain.BuiltinNextHopSelf, policychain.NextHopModeSelf},
		{"unchanged", policychain.BuiltinNextHopUnchanged, policychain.NextHopModeUnchanged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := ExportPolicyWithNextHop(mustPolicy(t, nil, routepolicy.FinalAccept), tc.ref)
			got, ok := staticExportNextHop(p)
			require.True(t, ok)
			require.Equal(t, tc.want, got.Mode)
			require.False(t, hasActiveFilter(p, directionExport), "static-only next-hop chain stays on fast path")
		})
	}

	dynamic := exportNextHopPolicy(t, []policychain.Member{
		{Name: exportNHFilter, Plugin: answerPlugin},
		{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
	})
	_, ok := staticExportNextHop(dynamic)
	require.False(t, ok)
	require.True(t, hasActiveFilter(dynamic, directionExport))

	uncompiled := mustPolicy(t, []routepolicy.Ref{{Name: "ext:unknown"}}, routepolicy.FinalAccept)
	_, ok = staticExportNextHop(uncompiled)
	require.False(t, ok)
	require.True(t, hasActiveFilter(uncompiled, directionExport))

	ingressBound := (filterapi.Policy{}).WithBoundFilters([]routepolicy.Ref{{Name: "loop-detection"}}, []bool{true})
	_, ok = staticExportNextHop(ingressBound)
	require.False(t, ok)
	require.True(t, hasActiveFilter(ingressBound, directionExport))
}

// TestExportNextHopChainConditionalAndFolding verifies conditional modify
// next-hop sets (<ip>, unchanged, self), last-write-wins explicit folding, and
// fill-if-unset trailing next-hop:auto across IPv4 and IPv6 on both forward
// rails.
//
// VALIDATES: AC-18 conditional export-chain next-hop intents and D-3 folding.
// PREVENTS: post-policy self rewrites clobbering explicit or unchanged export-chain decisions.
func TestExportNextHopChainConditionalAndFolding(t *testing.T) {
	explicit4 := netip.MustParseAddr("192.0.2.50")
	explicit4Second := netip.MustParseAddr("192.0.2.51")
	explicit6 := netip.MustParseAddr("2001:db8::50")
	explicit6Second := netip.MustParseAddr("2001:db8::51")

	for _, tc := range nextHopAutoFamilies() {
		explicit1, explicit2 := explicit4, explicit4Second
		if tc.family == family.IPv6Unicast {
			explicit1, explicit2 = explicit6, explicit6Second
		}
		for _, native := range []bool{false, true} {
			rail := nextHopAutoRailName(native)

			t.Run(tc.name+"/ebgp-explicit-vs-fallback/"+rail, func(t *testing.T) {
				f := nextHopAutoFixture(t, tc, true)
				matchNext := true
				members := []policychain.Member{
					{Name: exportNHFilter, Plugin: answerPlugin},
					{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
				}
				attachExportNextHopTransport(t, f, members, []string{"raw"}, []string{exportNHField}, func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
					if matchNext {
						return answerSet(exportNHField, strconv.Quote(explicit1.String()))
					}
					return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
				})

				// Matching route: explicit address must NOT be overwritten by trailing auto.
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
				updates := nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, _, _ := nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, explicit1.AsSlice(), hop)

				// Non-matching route: falls through to trailing next-hop:auto -> self on eBGP.
				f.conn.reset()
				matchNext = false
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.sibling, nil), native)
				updates = nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, _, _ = nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, tc.self.AsSlice(), hop)
			})

			t.Run(tc.name+"/ebgp-unchanged-vs-fallback/"+rail, func(t *testing.T) {
				f := nextHopAutoFixture(t, tc, true)
				matchNext := true
				members := []policychain.Member{
					{Name: exportNHFilter, Plugin: answerPlugin},
					{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
				}
				attachExportNextHopTransport(t, f, members, []string{"raw"}, []string{exportNHField}, func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
					if matchNext {
						return answerSet(exportNHField, strconv.Quote("unchanged"))
					}
					return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
				})

				// Matching route: unchanged suppresses trailing auto on eBGP.
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
				updates := nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, _, _ := nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, tc.received.AsSlice(), hop)

				// Non-matching route: trailing auto rewrites to self on eBGP.
				f.conn.reset()
				matchNext = false
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.sibling, nil), native)
				updates = nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, _, _ = nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, tc.self.AsSlice(), hop)
			})

			t.Run(tc.name+"/ibgp-self-vs-fallback/"+rail, func(t *testing.T) {
				f := nextHopAutoFixture(t, tc, true)
				f.destination.settings.PeerAS = f.destination.settings.LocalAS
				matchNext := true
				members := []policychain.Member{
					{Name: exportNHFilter, Plugin: answerPlugin},
					{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
				}
				attachExportNextHopTransport(t, f, members, []string{"raw"}, []string{exportNHField}, func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
					if matchNext {
						return answerSet(exportNHField, strconv.Quote("self"))
					}
					return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
				})

				// Matching route: conditional self rewrites on iBGP.
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
				updates := nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, _, _ := nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, tc.self.AsSlice(), hop)

				// Non-matching route: trailing auto preserves received next hop on iBGP.
				f.conn.reset()
				matchNext = false
				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.sibling, nil), native)
				updates = nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, _, _ = nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, tc.received.AsSlice(), hop)
			})

			t.Run(tc.name+"/last-write-wins-explicit/"+rail, func(t *testing.T) {
				f := nextHopAutoFixture(t, tc, true)
				members := []policychain.Member{
					{Name: exportNHFilter, Plugin: answerPlugin},
					{Name: exportNHFilter2, Plugin: answerPlugin},
					{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
				}
				attachExportNextHopTransport(t, f, members, []string{"raw"}, []string{exportNHField}, func(in rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
					if in.Filter == exportNHFilter {
						return answerSet(exportNHField, strconv.Quote(explicit1.String()))
					}
					return answerSet(exportNHField, strconv.Quote(explicit2.String()))
				})

				rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), native)
				updates := nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, _, _ := nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, explicit2.AsSlice(), hop)
			})
		}
	}
}

// TestExportNextHopChainRawOverrideProvenance verifies D-3 next-hop provenance
// checking when a whole-UPDATE raw override ends the export chain early.
//
// VALIDATES: AC-18 / D-3 raw-override next-hop byte provenance against received UPDATE.
// PREVENTS: trailing next-hop:auto overwriting a raw filter's deliberate next-hop rewrite or skipping an untouched received next hop.
func TestExportNextHopChainRawOverrideProvenance(t *testing.T) {
	explicit4 := netip.MustParseAddr("192.0.2.77")
	explicit6 := netip.MustParseAddr("2001:db8::77")

	for _, tc := range nextHopAutoFamilies() {
		explicit := explicit4
		if tc.family == family.IPv6Unicast {
			explicit = explicit6
		}
		for _, native := range []bool{false, true} {
			rail := nextHopAutoRailName(native)

			t.Run(tc.name+"/raw-unchanged-nh-rewrites-self/"+rail, func(t *testing.T) {
				f := nextHopAutoFixture(t, tc, true)
				body := nextHopAutoBody(t, tc, tc.prefix, nil)
				// Raw override edits MED only; next-hop bytes still equal the received UPDATE.
				rawReply := rawLibraryEdit(t, f.r, body, func(mods *filterapi.ModAccumulator) {
					mods.OpCopy(uint8(attribute.AttrMED), filterapi.AttrModSet, []byte{0, 0, 0, 55})
				})
				members := []policychain.Member{
					{Name: exportNHFilter, Plugin: answerPlugin},
					{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
				}
				attachExportNextHopTransport(t, f, members, []string{"raw"}, []string{"med"}, func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
					return rawReply
				})

				rfc4271ProofRelay(t, f, body, native)
				updates := nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, _, _ := nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, tc.self.AsSlice(), hop, "untouched raw next-hop must still receive trailing next-hop:auto on eBGP")
			})

			t.Run(tc.name+"/raw-changed-nh-preserved/"+rail, func(t *testing.T) {
				f := nextHopAutoFixture(t, tc, true)
				body := nextHopAutoBody(t, tc, tc.prefix, nil)
				rawReply := rawLibraryEdit(t, f.r, body, func(mods *filterapi.ModAccumulator) {
					if tc.family == family.IPv4Unicast {
						mods.OpCopy(uint8(attribute.AttrNextHop), filterapi.AttrModSet, explicit.AsSlice())
					} else {
						mods.OpCopy(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, explicit.AsSlice())
					}
				})
				members := []policychain.Member{
					{Name: exportNHFilter, Plugin: answerPlugin},
					{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
				}
				attachExportNextHopTransport(t, f, members, []string{"raw"}, []string{exportNHField}, func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
					return rawReply
				})

				rfc4271ProofRelay(t, f, body, native)
				updates := nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, _, _ := nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, explicit.AsSlice(), hop, "raw filter's rewritten next-hop must not be clobbered by trailing auto")
			})

			t.Run(tc.name+"/earlier-unchanged-before-raw-preserved/"+rail, func(t *testing.T) {
				f := nextHopAutoFixture(t, tc, true)
				body := nextHopAutoBody(t, tc, tc.prefix, nil)
				rawReply := rawLibraryEdit(t, f.r, body, func(mods *filterapi.ModAccumulator) {
					mods.OpCopy(uint8(attribute.AttrMED), filterapi.AttrModSet, []byte{0, 0, 0, 88})
				})
				members := []policychain.Member{
					{Name: policychain.BuiltinNextHopUnchanged},
					{Name: exportNHFilter, Plugin: answerPlugin},
					{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
				}
				attachExportNextHopTransport(t, f, members, []string{"raw"}, []string{"med"}, func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
					return rawReply
				})

				rfc4271ProofRelay(t, f, body, native)
				updates := nextHopAutoUpdates(t, f)
				require.Len(t, updates, 1)
				hop, _, _ := nextHopAutoFields(t, tc, updates[0])
				require.Equal(t, tc.received.AsSlice(), hop, "earlier next-hop:unchanged before raw override must suppress trailing auto")
			})
		}
	}
}

// TestExportNextHopChainMissingLocalWithholds verifies that conditional
// `modify { set { next-hop self; } }` withholds an announcement when the
// session has no local address, while non-matching iBGP routes still pass.
//
// VALIDATES: AC-18 dynamic next-hop self withholding on missing local address.
// PREVENTS: conditional self leaking a third-party next hop when no local endpoint exists.
func TestExportNextHopChainMissingLocalWithholds(t *testing.T) {
	tc := nextHopAutoFamilies()[0] // IPv4 unicast
	f := nextHopAutoFixture(t, tc, true)
	f.destination.settings.PeerAS = f.destination.settings.LocalAS // iBGP
	f.destination.settings.LocalAddress = netip.Addr{}
	f.destination.session.transport.Store(nil)

	matchSelf := true
	members := []policychain.Member{
		{Name: exportNHFilter, Plugin: answerPlugin},
		{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
	}
	attachExportNextHopTransport(t, f, members, []string{"raw"}, []string{exportNHField}, func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
		if matchSelf {
			return answerSet(exportNHField, strconv.Quote("self"))
		}
		return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
	})

	// Matching route requires self -> withheld because local address is absent.
	rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.prefix, nil), false)
	updates := nextHopAutoUpdates(t, f)
	for _, u := range updates {
		_, announced, _ := nextHopAutoFields(t, tc, u)
		require.Empty(t, announced, "conditional self with missing local address must withhold announcement")
	}

	// Non-matching route falls through to trailing auto on iBGP -> received next hop passes.
	f.conn.reset()
	matchSelf = false
	rfc4271ProofRelay(t, f, nextHopAutoBody(t, tc, tc.sibling, nil), false)
	updates = nextHopAutoUpdates(t, f)
	require.Len(t, updates, 1)
	hop, announced, _ := nextHopAutoFields(t, tc, updates[0])
	require.Equal(t, tc.received.AsSlice(), hop)
	require.Equal(t, tc.sibling, announced)
}

// TestExportNextHopChainFlowSpecRedirectIntent verifies dynamic FlowSpec 0x0800
// redirect-to-IP next-hop resolution under export-chain intents.
//
// VALIDATES: AC-18 FlowSpec redirect-to-IP intent dispatch (unchanged, self, auto, explicit, raw override).
// PREVENTS: removing PeerSettings.NextHopMode from breaking FlowSpec 0x0800 redirect rewriting.
func TestExportNextHopChainFlowSpecRedirectIntent(t *testing.T) {
	self4 := netip.MustParseAddr("192.0.2.10")
	explicit4 := netip.MustParseAddr("192.0.2.99")
	explicit6 := netip.MustParseAddr("2001:db8::99")

	ebgpFacts := &peerForwardFacts{
		isEBGP:         true,
		connectedLocal: self4,
		exportPolicy:   exportNextHopPolicy(t, []policychain.Member{{Name: exportNHFilter, Plugin: answerPlugin}}),
	}
	s := &PeerSettings{FlowSpecRedirectNextHop: true}
	precomputeNextHop(s, ebgpFacts)
	precomputeRedirectNextHop(s, ebgpFacts)

	require.True(t, ebgpFacts.redirectEnabled)
	require.False(t, ebgpFacts.hasStaticNextHop)

	keep := redirectForIntent(ebgpFacts, policychain.NextHopIntent{Mode: policychain.NextHopModeUnchanged})
	require.Equal(t, redirectKeep, keep.action)

	selfRes := redirectForIntent(ebgpFacts, policychain.NextHopIntent{Mode: policychain.NextHopModeSelf})
	require.Equal(t, redirectRewrite, selfRes.action)
	require.Equal(t, self4.AsSlice(), selfRes.hop(family.AFIIPv4))
	require.Nil(t, selfRes.hop(family.AFIIPv6))

	autoEBGP := redirectForIntent(ebgpFacts, policychain.NextHopIntent{Mode: policychain.NextHopModeAuto, AutoDefault: true})
	require.Equal(t, redirectRewrite, autoEBGP.action)
	require.Equal(t, self4.AsSlice(), autoEBGP.hop(family.AFIIPv4))

	exp4 := redirectForIntent(ebgpFacts, policychain.NextHopIntent{Mode: policychain.NextHopModeExplicit, Addr: explicit4})
	require.Equal(t, redirectRewrite, exp4.action)
	require.Equal(t, explicit4.AsSlice(), exp4.hop(family.AFIIPv4))
	require.Nil(t, exp4.hop(family.AFIIPv6))

	exp6 := redirectForIntent(ebgpFacts, policychain.NextHopIntent{Mode: policychain.NextHopModeExplicit, Addr: explicit6})
	require.Equal(t, redirectRewrite, exp6.action)
	require.Equal(t, explicit6.AsSlice(), exp6.hop(family.AFIIPv6))
	require.Nil(t, exp6.hop(family.AFIIPv4))

	ibgpFacts := *ebgpFacts
	ibgpFacts.isEBGP = false
	autoIBGP := redirectForIntent(&ibgpFacts, policychain.NextHopIntent{Mode: policychain.NextHopModeAuto, AutoDefault: true})
	require.Equal(t, redirectKeep, autoIBGP.action)

	rsFacts := *ebgpFacts
	rsFacts.rsClient = true
	autoRS := redirectForIntent(&rsFacts, policychain.NextHopIntent{Mode: policychain.NextHopModeAuto, AutoDefault: true})
	require.Equal(t, redirectKeep, autoRS.action)
}

// TestOriginatedNextHopExportChainPolicy proves D-5 on originated routes:
// auto-appended trailing next-hop:auto preserves originator-chosen next hops,
// explicit next-hop:self and modify { set { next-hop <ip> } } apply uniformly,
// and defaultOriginationNextHop honors static next-hop:unchanged vs default.
//
// VALIDATES: AC-19 / D-5 originated route next-hop preservation and explicit export rewrite.
// PREVENTS: trailing next-hop:auto clobbering static/ExaBGP/API originated next hops on eBGP.
func TestOriginatedNextHopExportChainPolicy(t *testing.T) {
	explicit4 := netip.MustParseAddr("192.0.2.55")
	explicit6 := netip.MustParseAddr("2001:db8::55")

	for _, tc := range nextHopAutoFamilies() {
		explicit := explicit4
		if tc.family == family.IPv6Unicast {
			explicit = explicit6
		}
		origBody := nextHopAutoBody(t, tc, tc.prefix, nil)

		t.Run(tc.name+"/trailing-auto-preserves-originated", func(t *testing.T) {
			r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
			p := NewPeer(&PeerSettings{
				Address:      netip.MustParseAddr("192.0.2.2"),
				LocalAddress: tc.self,
				LocalAS:      65000,
				PeerAS:       65002,
				ExportPolicy: exportNextHopPolicy(t, []policychain.Member{
					{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
				}),
			})
			facts := new(peerForwardFacts)
			p.captureOriginatedFacts(facts)

			suppress, override, _ := r.exportFilterFromFacts(p, facts, true, origBody)
			require.False(t, suppress)
			if override != nil {
				require.Equal(t, payloadNextHop(origBody), payloadNextHop(override))
			}
		})

		t.Run(tc.name+"/explicit-self-rewrites-originated", func(t *testing.T) {
			r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
			p := NewPeer(&PeerSettings{
				Address:      netip.MustParseAddr("192.0.2.2"),
				LocalAddress: tc.self,
				LocalAS:      65000,
				PeerAS:       65002,
				ExportPolicy: ExportPolicyWithNextHop(mustPolicy(t, nil, routepolicy.FinalAccept), policychain.BuiltinNextHopSelf),
			})
			facts := new(peerForwardFacts)
			p.captureOriginatedFacts(facts)

			suppress, override, _ := r.exportFilterFromFacts(p, facts, true, origBody)
			require.False(t, suppress)
			require.NotNil(t, override)
			got := payloadNextHop(override)
			if tc.family == family.IPv4Unicast {
				require.Equal(t, tc.self, got.legacy)
			} else {
				require.Equal(t, tc.self, got.mp)
			}
		})

		t.Run(tc.name+"/explicit-self-missing-local-suppresses-originated", func(t *testing.T) {
			r := &Reactor{attrModHandlers: attrModHandlersWithDefaults()}
			p := NewPeer(&PeerSettings{
				Address:      netip.MustParseAddr("192.0.2.2"),
				LocalAS:      65000,
				PeerAS:       65002,
				ExportPolicy: ExportPolicyWithNextHop(mustPolicy(t, nil, routepolicy.FinalAccept), policychain.BuiltinNextHopSelf),
			})
			facts := new(peerForwardFacts)
			p.captureOriginatedFacts(facts)

			suppress, _, _ := r.exportFilterFromFacts(p, facts, true, origBody)
			require.True(t, suppress, "explicit next-hop:self without local address must suppress originated announcement")
		})

		t.Run(tc.name+"/conditional-modify-explicit-vs-trailing-auto", func(t *testing.T) {
			matchModify := true
			r := &Reactor{
				attrModHandlers: attrModHandlersWithDefaults(),
				filterTransportSeam: &answerTransport{
					reads: []string{"raw"},
					sets:  []string{exportNHField},
					answer: func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
						if matchModify {
							return answerSet(exportNHField, strconv.Quote(explicit.String()))
						}
						return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
					},
				},
			}
			p := NewPeer(&PeerSettings{
				Address:      netip.MustParseAddr("192.0.2.2"),
				LocalAddress: tc.self,
				LocalAS:      65000,
				PeerAS:       65002,
				ExportPolicy: exportNextHopPolicy(t, []policychain.Member{
					{Name: exportNHFilter, Plugin: answerPlugin},
					{Name: policychain.BuiltinNextHopAuto, AutoDefault: true},
				}),
			})
			facts := new(peerForwardFacts)
			p.captureOriginatedFacts(facts)

			// Matching originated route gets explicit <ip>.
			suppress, override, _ := r.exportFilterFromFacts(p, facts, true, origBody)
			require.False(t, suppress)
			require.NotNil(t, override)
			got := payloadNextHop(override)
			if tc.family == family.IPv4Unicast {
				require.Equal(t, explicit, got.legacy)
			} else {
				require.Equal(t, explicit, got.mp)
			}

			// Non-matching originated route hits trailing auto -> preserved!
			matchModify = false
			suppress, override, _ = r.exportFilterFromFacts(p, facts, true, origBody)
			require.False(t, suppress)
			if override != nil {
				require.Equal(t, payloadNextHop(origBody), payloadNextHop(override))
			}
		})
	}

	t.Run("defaultOriginationNextHop", func(t *testing.T) {
		cand := netip.MustParseAddr("198.51.100.1")
		peerAddr := netip.MustParseAddr("192.0.2.2")

		// Default eBGP -> self.
		ebgpDefault := &PeerSettings{Address: peerAddr, LocalAS: 65000, PeerAS: 65002}
		chosen, err := defaultOriginationNextHop(ebgpDefault, cand)
		require.NoError(t, err)
		require.Equal(t, bgptypes.NextHopSelf, chosen.Policy)

		// Static next-hop:unchanged on eBGP -> preserves candidate.
		ebgpUnchanged := &PeerSettings{
			Address:      peerAddr,
			LocalAS:      65000,
			PeerAS:       65002,
			ExportPolicy: ExportPolicyWithNextHop(mustPolicy(t, nil, routepolicy.FinalAccept), policychain.BuiltinNextHopUnchanged),
		}
		chosen, err = defaultOriginationNextHop(ebgpUnchanged, cand)
		require.NoError(t, err)
		require.Equal(t, bgptypes.NewNextHopExplicit(cand), chosen)

		// Static next-hop:self on iBGP -> self.
		ibgpSelf := &PeerSettings{
			Address:      peerAddr,
			LocalAS:      65000,
			PeerAS:       65000,
			ExportPolicy: ExportPolicyWithNextHop(mustPolicy(t, nil, routepolicy.FinalAccept), policychain.BuiltinNextHopSelf),
		}
		chosen, err = defaultOriginationNextHop(ibgpSelf, cand)
		require.NoError(t, err)
		require.Equal(t, bgptypes.NextHopSelf, chosen.Policy)

		// Default iBGP keeps normal candidate, uses self for link-local or peer-own.
		ibgpDefault := &PeerSettings{Address: peerAddr, LocalAS: 65000, PeerAS: 65000}
		chosen, err = defaultOriginationNextHop(ibgpDefault, cand)
		require.NoError(t, err)
		require.Equal(t, bgptypes.NewNextHopExplicit(cand), chosen)

		chosen, err = defaultOriginationNextHop(ibgpDefault, netip.MustParseAddr("fe80::1"))
		require.NoError(t, err)
		require.Equal(t, bgptypes.NextHopSelf, chosen.Policy)

		chosen, err = defaultOriginationNextHop(ibgpDefault, peerAddr)
		require.NoError(t, err)
		require.Equal(t, bgptypes.NextHopSelf, chosen.Policy)
	})
}

// TestPolicyDryRunTracesBuiltinNextHop proves `show policy test` runs
// traceLibraryChain on static-only built-in next-hop:* export policies and
// traces each built-in step alongside explicit modify filters.
//
// VALIDATES: AC-20 / show policy test dry-run visibility of built-in next-hop:* steps.
// PREVENTS: static fast-path bypass hiding built-in next-hop steps from dry-run traces.
func TestPolicyDryRunTracesBuiltinNextHop(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	settings.ExportPolicy = ExportPolicyWithNextHop(mustPolicy(t, nil, routepolicy.FinalAccept), policychain.BuiltinNextHopSelf)
	peer := NewPeer(settings)
	r.peers[settings.PeerKey()] = peer

	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(
		peer.sourceOwner, directionExport, "", libraryTestBody(100), true, "learned")
	require.NoError(t, err)
	require.Equal(t, dryRunActionAccept, result.Action)
	require.Len(t, result.Trace, 1)
	require.Equal(t, plugin.PolicyTraceEntry{
		Filter:    "self",
		Canonical: policychain.BuiltinNextHopSelf,
		Action:    dryRunActionModify,
		Delta:     "next-hop self",
		TextAfter: result.TextBefore,
	}, result.Trace[0])
}

// TestExportNextHopChainCrossFamilyExplicitFallsBackToTrailingAuto proves that
// an explicit IPv6 next-hop set on a classic IPv4 unicast route does not
// suppress trailing auto-default next-hop:auto on ordinary eBGP (RFC 4271 §5.1.3).
//
// VALIDATES: cross-family explicit next-hop falls back to trailing auto on eBGP.
// PREVENTS: a matched IPv6 modify rule leaking a received third-party IPv4 NEXT_HOP on eBGP.
func TestExportNextHopChainCrossFamilyExplicitFallsBackToTrailingAuto(t *testing.T) {
	t.Parallel()
	const localSelf = "192.0.2.254"
	tc := nextHopAutoFamilies()[0]
	var facts peerForwardFacts
	facts.isEBGP = true
	facts.selfMode = nhModeSelf4
	copy(facts.nhLegacy[:], netip.MustParseAddr(localSelf).AsSlice())

	srcPayload := nextHopAutoBody(t, tc, tc.prefix, nil)
	baseNH := payloadNextHop(srcPayload)
	var mods filterapi.ModAccumulator
	withheld := applyEgressNextHopIntent(
		&facts,
		policychain.NextHopIntent{Mode: policychain.NextHopModeExplicit, Addr: netip.MustParseAddr("2001:db8::66")},
		false, false, true,
		baseNH, baseNH, srcPayload, srcPayload, &mods,
	)
	require.False(t, withheld)
	eff := effectiveNextHop(&mods, baseNH)
	require.Equal(t, netip.MustParseAddr(localSelf), eff.legacy)
}

// TestExportNextHopChainLaterRawOverrideBeatsEarlierSelf proves D-3 last-write-
// wins when an earlier next-hop:self precedes a raw full-UPDATE override: a raw
// override that deliberately rewrites NEXT_HOP wins, while a raw override that
// leaves NEXT_HOP unchanged still receives next-hop:self.
//
// VALIDATES: D-3 last-write-wins provenance for earlier next-hop:self + later raw override.
// PREVENTS: an earlier next-hop:self clobbering a later raw filter's deliberate NEXT_HOP rewrite.
func TestExportNextHopChainLaterRawOverrideBeatsEarlierSelf(t *testing.T) {
	t.Parallel()
	const (
		rawNH     = "192.0.2.77"
		localSelf = "192.0.2.254"
	)
	tc := nextHopAutoFamilies()[0]
	var facts peerForwardFacts
	facts.isEBGP = true
	facts.selfMode = nhModeSelf4
	copy(facts.nhLegacy[:], netip.MustParseAddr(localSelf).AsSlice())

	srcPayload := nextHopAutoBody(t, tc, tc.prefix, nil)
	srcNH := payloadNextHop(srcPayload)

	t.Run("raw_rewrote_nexthop_wins", func(t *testing.T) {
		var rewriteMods filterapi.ModAccumulator
		rewriteMods.OpCopy(uint8(attribute.AttrNextHop), filterapi.AttrModSet, netip.MustParseAddr(rawNH).AsSlice())
		r := New(&Config{})
		rawPayload, _, fail := buildModifiedPayload(srcPayload, &rewriteMods, r.attrModHandlers, nil, nil)
		require.False(t, fail.failed())
		require.NotNil(t, rawPayload)
		baseNH := payloadNextHop(rawPayload)
		require.Equal(t, netip.MustParseAddr(rawNH), baseNH.legacy)
		var mods filterapi.ModAccumulator
		withheld := applyEgressNextHopIntent(
			&facts,
			policychain.NextHopIntent{Mode: policychain.NextHopModeSelf},
			true, false, false,
			srcNH, baseNH, srcPayload, rawPayload, &mods,
		)
		require.False(t, withheld)
		eff := effectiveNextHop(&mods, baseNH)
		require.Equal(t, netip.MustParseAddr(rawNH), eff.legacy)
	})

	t.Run("later_self_after_raw_wins", func(t *testing.T) {
		var rewriteMods filterapi.ModAccumulator
		rewriteMods.OpCopy(uint8(attribute.AttrNextHop), filterapi.AttrModSet, netip.MustParseAddr(rawNH).AsSlice())
		r := New(&Config{})
		rawPayload, _, fail := buildModifiedPayload(srcPayload, &rewriteMods, r.attrModHandlers, nil, nil)
		require.False(t, fail.failed())
		require.NotNil(t, rawPayload)
		baseNH := payloadNextHop(rawPayload)
		var mods filterapi.ModAccumulator
		withheld := applyEgressNextHopIntent(
			&facts,
			policychain.NextHopIntent{Mode: policychain.NextHopModeSelf},
			true, true, false,
			srcNH, baseNH, srcPayload, rawPayload, &mods,
		)
		require.False(t, withheld)
		eff := effectiveNextHop(&mods, baseNH)
		require.Equal(t, netip.MustParseAddr(localSelf), eff.legacy)
	})

	t.Run("raw_left_nexthop_untouched_gets_self", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		withheld := applyEgressNextHopIntent(
			&facts,
			policychain.NextHopIntent{Mode: policychain.NextHopModeSelf},
			true, false, false,
			srcNH, srcNH, srcPayload, srcPayload, &mods,
		)
		require.False(t, withheld)
		eff := effectiveNextHop(&mods, srcNH)
		require.Equal(t, netip.MustParseAddr(localSelf), eff.legacy)
	})
}
