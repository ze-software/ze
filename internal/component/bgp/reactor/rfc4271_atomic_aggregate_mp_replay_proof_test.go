// Design: docs/architecture/bgp/egress-attribute-rules.md -- marked-route policy admission across MP NLRI, native families, and stored-route replay.
// Related: rfc4271_atomic_aggregate_policy_rewrite_test.go -- legacy IPv4 raw-export deaggregation proof.
// Related: rfc4271_atomic_policy_admission_test.go -- shared MP and relay fixtures.
// RFC: rfc/short/rfc4271.md -- Section 5.1.6 and Section 9.1.4.
package reactor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC4271AtomicAggregateMPNLRIRelayPreservesExactPrefix proves that both
// ordinary and native route-server relay rails preserve exact received IPv6
// MP_REACH_NLRI /48 prefixes (standalone and mixed with legacy IPv4 /24) with
// ATOMIC_AGGREGATE intact across export attribute rebuilds and exact subsets.
//
// RFC 4271 Section 5.1.6: "A BGP speaker that receives a route with the
// ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific
// (as defined in 9.1.4) when advertising this route to other BGP speakers."
// RFC requirement: RFC4271-5.1.6-1 positive -- general and native route-server
// relay preserve exact received IPv6 MP_REACH_NLRI /48 prefixes (standalone
// and mixed with legacy IPv4 /24) with ATOMIC_AGGREGATE intact when export
// policy rebuilds MED or retains an exact subset while withdrawing omitted keys.
func TestRFC4271AtomicAggregateMPNLRIRelayPreservesExactPrefix(t *testing.T) {
	first := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 1}
	second := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2}
	both := append(bytes.Clone(first), second...)
	legacy := []byte{24, 10, 42, 0}

	for _, rs := range []bool{false, true} {
		for _, mixed := range []bool{false, true} {
			mode := "mp-only"
			var legacyNLRI []byte
			if mixed {
				mode = "mixed-ipv4-ipv6"
				legacyNLRI = legacy
			}
			for _, tc := range []struct {
				name    string
				kept    []byte
				omitted []byte
			}{
				{"rebuild-equal", both, nil},
				{"exact-subset", second, first},
			} {
				t.Run(atomicAdmissionRailName(rs)+"/"+mode+"/"+tc.name, func(t *testing.T) {
					f := atomicAdmissionMPFixture(t)
					initial := atomicAdmissionMPBody(t, legacyNLRI, both, nil, true, 10)
					rfc4271AtomicPolicyRelay(t, f, initial, rs)
					before := len(f.conn.written())
					require.Positive(t, before, "initial marked MP route must reach the socket")

					output := atomicAdmissionMPBody(t, legacyNLRI, tc.kept, nil, true, 77)
					exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
					}, f.destination)
					rfc4271AtomicPolicyRelay(t, f, atomicAdmissionMPBody(t, legacyNLRI, both, nil, true, 20), rs)

					bodies := updateBodies(t, f.conn.written()[before:])
					require.NotEmpty(t, bodies, "rebuilt marked MP output must reach the socket")
					var announcedLegacy, announcedMP, withdrawnMP []byte
					for _, body := range bodies {
						update, err := message.UnpackUpdate(body)
						require.NoError(t, err)
						require.Empty(t, update.WithdrawnRoutes)
						announcedLegacy = append(announcedLegacy, update.NLRI...)
						_, _, reach, hasReach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
						if hasReach {
							require.GreaterOrEqual(t, len(reach), 5)
							require.Equal(t, []byte{0, 2, 1}, reach[:3])
							announcedMP = append(announcedMP, reach[5+int(reach[3]):]...)
							_, _, marker, marked := attribute.AttrFind(update.PathAttributes, attribute.AttrAtomicAggregate)
							require.True(t, marked, "ATOMIC_AGGREGATE must remain present on rebuilt MP output")
							require.Empty(t, marker)
							med := payloadMED(body)
							require.True(t, med.present)
							require.Equal(t, uint32(77), binary.BigEndian.Uint32(med.raw))
						}
						_, _, unreach, hasUnreach := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI)
						if hasUnreach {
							require.GreaterOrEqual(t, len(unreach), 3)
							require.Equal(t, []byte{0, 2, 1}, unreach[:3])
							withdrawnMP = append(withdrawnMP, unreach[3:]...)
						}
					}
					require.Equal(t, legacyNLRI, announcedLegacy)
					require.Equal(t, tc.kept, announcedMP, "exact IPv6 /48 NLRI must be preserved")
					require.Equal(t, tc.omitted, withdrawnMP, "omitted IPv6 /48 key must be withdrawn")
				})
			}
		}
	}
}

// TestRFC4271AtomicAggregateMPNLRIAndNativeFamiliesRefuseDeaggregation proves
// that raw export-policy /48-to-/49 rewrites of received marked IPv6
// MP_REACH_NLRI routes do not reach the final socket on ordinary or native
// route-server rails (with ATOMIC_AGGREGATE retained or stripped), withdraw a
// prior /48 advertisement via MP_UNREACH_NLRI, allow unmarked /48-to-/49
// controls, and reject new native keys across every registered family.
//
// RFC 4271 Section 5.1.6: "A BGP speaker that receives a route with the
// ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific
// (as defined in 9.1.4) when advertising this route to other BGP speakers."
// RFC requirement: RFC4271-5.1.6-1 negative -- raw export-policy /48-to-/49
// rewrites of a received marked IPv6 MP_REACH_NLRI route are refused on
// ordinary and native route-server rails whether ATOMIC_AGGREGATE is retained
// or stripped, withdrawing any prior /48 advertisement via MP_UNREACH_NLRI
// while unmarked /48-to-/49 controls reach the socket; every registered native
// family rejects newly introduced identities with and without ADD-PATH.
func TestRFC4271AtomicAggregateMPNLRIAndNativeFamiliesRefuseDeaggregation(t *testing.T) {
	original := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 1}
	specific := []byte{49, 0x20, 1, 0x0d, 0xb8, 0, 1, 0}
	cover := netip.MustParsePrefix("2001:db8:1::/48")

	for _, rs := range []bool{false, true} {
		for _, prior := range []bool{false, true} {
			priorName := "fresh"
			if prior {
				priorName = "prior-advertised"
			}
			for _, tc := range []struct {
				name         string
				atomic       bool
				outputAtomic bool
			}{
				{"unmarked-control", false, false},
				{"atomic-aggregate", true, true},
				{"atomic-aggregate-stripped", true, false},
			} {
				t.Run(atomicAdmissionRailName(rs)+"/"+priorName+"/"+tc.name, func(t *testing.T) {
					f := atomicAdmissionMPFixture(t)
					if prior {
						rfc4271AtomicPolicyRelay(t, f, atomicAdmissionMPBody(t, nil, original, nil, tc.atomic, 10), rs)
						require.Positive(t, len(f.conn.written()), "prior IPv6 /48 must reach the socket")
					}
					before := len(f.conn.written())
					rewritten := atomicAdmissionMPBody(t, nil, specific, nil, tc.outputAtomic, 77)
					exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rewritten}
					}, f.destination)
					rfc4271AtomicPolicyRelay(t, f, atomicAdmissionMPBody(t, nil, original, nil, tc.atomic, 20), rs)
					transport, ok := f.r.filterTransportSeam.(*answerTransport)
					require.True(t, ok)
					require.Equal(t, 1, transport.calls(), "export policy must run on the MP_REACH_NLRI route")

					after := updateBodies(t, f.conn.written()[before:])
					if !tc.atomic {
						require.Len(t, after, 1, "unmarked IPv6 /48-to-/49 control must reach the socket")
						update, err := message.UnpackUpdate(after[0])
						require.NoError(t, err)
						_, _, reach, present := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
						require.True(t, present)
						require.Equal(t, []byte{0, 2, 1}, reach[:3])
						require.Equal(t, specific, reach[5+int(reach[3]):])
						return
					}

					for _, body := range after {
						update, err := message.UnpackUpdate(body)
						require.NoError(t, err)
						_, _, reach, present := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
						if !present {
							continue
						}
						require.GreaterOrEqual(t, len(reach), 5)
						nlriBytes := reach[5+int(reach[3]):]
						for len(nlriBytes) > 0 {
							bits := int(nlriBytes[0])
							require.LessOrEqual(t, bits, 128)
							octets := (bits + 7) / 8
							require.GreaterOrEqual(t, len(nlriBytes), 1+octets)
							var raw [16]byte
							copy(raw[:], nlriBytes[1:1+octets])
							prefix := netip.PrefixFrom(netip.AddrFrom16(raw), bits).Masked()
							if cover.Contains(prefix.Addr()) {
								require.LessOrEqual(t, prefix.Bits(), cover.Bits(),
									"received ATOMIC_AGGREGATE IPv6 route %s was deaggregated to %s", cover, prefix)
							}
							nlriBytes = nlriBytes[1+octets:]
						}
					}

					events := ownershipRailRead(t, f.conn.written()[before:], f.destination.sendCtx.Load())
					if !prior {
						require.Empty(t, events, "fresh marked IPv6 refusal must neither announce /49 nor emit spurious withdrawal")
						return
					}
					require.Equal(t, []ownershipRailEvent{{
						family:   family.IPv6Unicast,
						key:      string(original),
						withdraw: true,
					}}, events, "prior IPv6 /48 advertisement must be withdrawn on refused deaggregation")
				})
			}
		}
	}

	// Native-family census across every registered family, with and without ADD-PATH.
	t.Run("all-registered-native-families", func(t *testing.T) {
		family.RegisterTestFamilies()
		first := atomicPolicyNativeFamilies(1)
		second := atomicPolicyNativeFamilies(2)
		extra := atomicPolicyNativeFamilies(3)
		for _, name := range family.RegisteredFamilyNames() {
			fam, ok := family.LookupFamily(name)
			require.True(t, ok, "registered family %q must resolve", name)
			a, hasA := first[fam]
			require.True(t, hasA, "registered family %s must have a native fixture", name)
			b, c := second[fam], extra[fam]
			for _, addPath := range []bool{false, true} {
				ctx := atomicPolicyContext(t, fam, addPath)
				one, two, inserted := a, b, c
				if addPath {
					one = atomicPolicyPath(0, a)
					two = atomicPolicyPath(7, b)
					inserted = atomicPolicyPath(9, c)
				}
				input := atomicPolicyMP(fam, bytes.Join([][]byte{one, two}, nil), true)
				atomicPolicyWant(t, input, atomicPolicyMP(fam, two, true), ctx, true)
				atomicPolicyWant(t, input, atomicPolicyMP(fam, inserted, true), ctx, false)
			}
		}
	})
}

// TestRFC4271AtomicAggregateStoredReplayRefusesDeaggregationAndWithdraws proves
// that after the live recent-UPDATE cache expires, stored-route replay through
// RelayStoredRoute enforces Section 5.1.6 for both legacy IPv4 (/24-to-/25) and
// IPv6 MP_REACH_NLRI (/48-to-/49) routes: marked deaggregation rewrites (marker
// retained or stripped) never reach the socket and withdraw the prior route,
// while unmarked deaggregation and marked attribute-only replay succeed.
//
// RFC 4271 Section 5.1.6: "A BGP speaker that receives a route with the
// ATOMIC_AGGREGATE attribute MUST NOT make any NLRI of that route more specific
// (as defined in 9.1.4) when advertising this route to other BGP speakers."
// RFC requirement: RFC4271-5.1.6-1 negative -- stored-route replay after live
// cache expiry refuses raw export-policy deaggregation for legacy IPv4
// (/24-to-/25) and IPv6 MP_REACH_NLRI (/48-to-/49) routes whether
// ATOMIC_AGGREGATE is retained or stripped, withdrawing the previously
// advertised route while permitting unmarked deaggregation and marked
// attribute-only replay.
func TestRFC4271AtomicAggregateStoredReplayRefusesDeaggregationAndWithdraws(t *testing.T) {
	v4Orig := []byte{24, 10, 42, 0}
	v4Spec := []byte{25, 10, 42, 0, 0}
	v6Orig := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 1}
	v6Spec := []byte{49, 0x20, 1, 0x0d, 0xb8, 0, 1, 0}
	v6NH := "20010db8000000000000000000000001"

	for _, famCase := range []struct {
		name     string
		fam      family.Family
		orig     []byte
		specific []byte
		nhHex    string
		build    func(t *testing.T, nlri []byte, marked bool, med byte) []byte
	}{
		{
			name:     "ipv4-unicast",
			fam:      family.IPv4Unicast,
			orig:     v4Orig,
			specific: v4Spec,
			nhHex:    "c612e701",
			build: func(_ *testing.T, nlri []byte, marked bool, med byte) []byte {
				return rfc4271ProofRelayBody(nlri, marked, med)
			},
		},
		{
			name:     "ipv6-unicast-mp",
			fam:      family.IPv6Unicast,
			orig:     v6Orig,
			specific: v6Spec,
			nhHex:    v6NH,
			build: func(t *testing.T, nlri []byte, marked bool, med byte) []byte {
				return atomicAdmissionMPBody(t, nil, nlri, nil, marked, med)
			},
		},
	} {
		for _, tc := range []struct {
			name         string
			atomic       bool
			outputNLRI   []byte
			outputAtomic bool
			expectAccept bool
		}{
			{"marked-attribute-only", true, famCase.orig, true, true},
			{"unmarked-deagg-control", false, famCase.specific, false, true},
			{"marked-deagg-retained", true, famCase.specific, true, false},
			{"marked-deagg-stripped", true, famCase.specific, false, false},
		} {
			t.Run(famCase.name+"/"+tc.name, func(t *testing.T) {
				f := atomicAdmissionMPFixture(t)
				initial := famCase.build(t, famCase.orig, tc.atomic, 10)
				sections, err := wire.ParseUpdateSections(initial)
				require.NoError(t, err)
				storedAttrs := storedReplayBaseAttrs(t, sections.Attrs(initial))

				id := f.receive(t, initial)
				f.forward(t, id)
				f.drain(t)
				require.False(t, f.r.recentUpdates.Contains(id), "stored-route replay must outlive the transient cache")
				before := len(f.conn.written())
				require.Positive(t, before, "prior advertisement must reach the socket before replay")

				rewritten := famCase.build(t, tc.outputNLRI, tc.outputAtomic, 77)
				exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
					return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rewritten}
				}, f.destination)

				stored := rpc.StoredRoute{
					SourcePeer:         f.source.Settings().Address.String(),
					SourceOwner:        f.source.sourceOwner,
					SourceReceipt:      f.source.currentSession().sourceReceiptID(),
					SourceConnectionID: f.source.currentSession().connectionID,
					SourcePolicyEpoch:  f.source.sourcePolicyEpoch.Load(),
					MsgID:              id,
					Family:             famCase.fam.String(),
					AttrHex:            hex.EncodeToString(storedAttrs),
					NextHopHex:         famCase.nhHex,
					NLRIHex:            hex.EncodeToString(famCase.orig),
					NLRIFraming:        rpc.NLRIFramingPrefixOnly,
				}
				require.NoError(t, (&reactorAPIAdapter{r: f.r}).RelayStoredRoute(
					f.destination.Settings().Address, []rpc.StoredRoute{stored}, plugin.ProcessSender("aigp-forwarder")))
				f.drain(t)

				events := ownershipRailRead(t, f.conn.written()[before:], f.destination.sendCtx.Load())
				if tc.expectAccept {
					require.Equal(t, []ownershipRailEvent{{
						family:   famCase.fam,
						key:      string(tc.outputNLRI),
						origin:   77,
						withdraw: false,
					}}, events, "accepted replay must emit the expected route")
					bodies := updateBodies(t, f.conn.written()[before:])
					require.Len(t, bodies, 1)
					med := payloadMED(bodies[0])
					require.True(t, med.present)
					require.Equal(t, uint32(77), binary.BigEndian.Uint32(med.raw))
					if tc.atomic {
						update, unpackErr := message.UnpackUpdate(bodies[0])
						require.NoError(t, unpackErr)
						_, _, _, marked := attribute.AttrFind(update.PathAttributes, attribute.AttrAtomicAggregate)
						require.True(t, marked)
					}
					return
				}
				require.Equal(t, []ownershipRailEvent{{
					family:   famCase.fam,
					key:      string(famCase.orig),
					withdraw: true,
				}}, events, "refused marked deaggregation during stored replay must withdraw the prior route without advertising the more-specific prefix")
			})
		}
	}
}

// storedReplayBaseAttrs strips MP_REACH_NLRI/MP_UNREACH_NLRI from raw path
// attributes because buildRelayUpdate reconstructs MP_REACH_NLRI from the
// stored family, NextHopHex and NLRIHex fields.
func storedReplayBaseAttrs(t *testing.T, raw []byte) []byte {
	t.Helper()
	var out []byte
	for len(raw) > 0 {
		_, code, length, header, err := attribute.ParseHeader(raw)
		require.NoError(t, err)
		end := header + int(length)
		require.LessOrEqual(t, end, len(raw))
		if code != attribute.AttrMPReachNLRI && code != attribute.AttrMPUnreachNLRI {
			out = append(out, raw[:end]...)
		}
		raw = raw[end:]
	}
	return out
}
