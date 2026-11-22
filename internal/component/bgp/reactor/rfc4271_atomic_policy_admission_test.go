// Design: docs/architecture/bgp/structural-forwarding.md -- policy admission precedes relay dispatch.
// Related: rfc4271_atomic_aggregate_policy_rewrite_test.go -- the original six-case wire regression.
// RFC: rfc/short/rfc4271.md -- Section 5.1.6; additional identity refusals are release policy.
// RFC naming: untagged -- admission integration probes without a producer-break record or root coverage claim.
package reactor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAtomicPolicyAdmissionRetainsUsefulOutput reads both final socket rails:
// the exact received destinations may be reordered or reduced, with a useful
// MED change. Rejecting every marked UPDATE cannot satisfy these assertions.
func TestAtomicPolicyAdmissionRetainsUsefulOutput(t *testing.T) {
	first := []byte{24, 10, 42, 0}
	second := []byte{24, 10, 43, 0}
	input := append(append([]byte(nil), first...), second...)
	for _, rs := range []bool{false, true} {
		for _, row := range []struct {
			name string
			nlri []byte
		}{
			{"equal", input},
			{"subset", second},
			{"reordered", append(append([]byte(nil), second...), first...)},
		} {
			t.Run(atomicAdmissionRailName(rs)+"/"+row.name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				rewritten := rfc4271ProofRelayBody(row.nlri, true, 77)
				exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
					return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rewritten}
				}, f.destination)
				rfc4271AtomicPolicyRelay(t, f, rfc4271ProofRelayBody(input, true, 10), rs)
				written := aigpSocketBodies(t, f.conn)
				require.Len(t, written, 1)
				out, err := message.UnpackUpdate(written[0])
				require.NoError(t, err)
				require.Equal(t, row.nlri, out.NLRI)
				require.Empty(t, out.WithdrawnRoutes)
				_, _, marker, present := attribute.AttrFind(out.PathAttributes, attribute.AttrAtomicAggregate)
				require.True(t, present)
				require.Empty(t, marker)
				med := payloadMED(written[0])
				require.True(t, med.present)
				require.Equal(t, uint32(77), binary.BigEndian.Uint32(med.raw))
			})
		}
	}
}

// TestAtomicPolicyAdmissionRefusalWithdrawsSource first advertises the source
// key, then reads the exact withdrawal after a refused replacement. A fresh
// refusal must stay quiet. Shorter and disjoint changes are the conservative
// release restriction, not a claim that RFC 4271 forbids aggregation.
func TestAtomicPolicyAdmissionRefusalWithdrawsSource(t *testing.T) {
	original := []byte{24, 10, 42, 0}
	for _, rs := range []bool{false, true} {
		for _, row := range []struct {
			name   string
			nlri   []byte
			marker bool
		}{
			{"same-key-marker-strip", original, false},
			{"more-specific", []byte{25, 10, 42, 0, 0}, true},
			{"less-specific-release-restriction", []byte{16, 10, 42}, true},
			{"disjoint-release-restriction", []byte{24, 10, 99, 0}, true},
		} {
			for _, prior := range []bool{false, true} {
				name := "never-advertised"
				if prior {
					name = "previously-advertised"
				}
				t.Run(atomicAdmissionRailName(rs)+"/"+row.name+"/"+name, func(t *testing.T) {
					f := newAIGPReplayFixture(t, nil)
					if prior {
						rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(original, true, 10), rs)
						require.Len(t, aigpSocketBodies(t, f.conn), 1)
					}
					rewritten := rfc4271ProofRelayBody(row.nlri, row.marker, 77)
					exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rewritten}
					}, f.destination)
					rfc4271AtomicPolicyRelay(t, f, rfc4271ProofRelayBody(original, true, 20), rs)
					written := aigpSocketBodies(t, f.conn)
					if !prior {
						require.Empty(t, written, "no prior recipient ownership authorizes a withdrawal")
						return
					}
					require.Len(t, written, 2)
					out, err := message.UnpackUpdate(written[1])
					require.NoError(t, err)
					require.Equal(t, original, out.WithdrawnRoutes, "withdraw the received key, not the refused answer")
					require.Empty(t, out.NLRI)
					require.Empty(t, out.PathAttributes)
				})
			}
		}
	}
}

// TestAtomicPolicyAdmissionUnmarkedRewriteAddsMarker proves the restriction
// is conditional on the chain input: an unmarked receipt may be rewritten and
// given the marker by the same member, on both final socket rails.
func TestAtomicPolicyAdmissionUnmarkedRewriteAddsMarker(t *testing.T) {
	original := []byte{24, 10, 42, 0}
	specific := []byte{25, 10, 42, 0, 0}
	for _, rs := range []bool{false, true} {
		t.Run(atomicAdmissionRailName(rs), func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			rewritten := rfc4271ProofRelayBody(specific, true, 77)
			exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
				return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rewritten}
			}, f.destination)
			rfc4271AtomicPolicyRelay(t, f, rfc4271ProofRelayBody(original, false, 10), rs)
			written := aigpSocketBodies(t, f.conn)
			require.Len(t, written, 1)
			out, err := message.UnpackUpdate(written[0])
			require.NoError(t, err)
			require.Equal(t, specific, out.NLRI)
			_, _, _, present := attribute.AttrFind(out.PathAttributes, attribute.AttrAtomicAggregate)
			require.True(t, present)
		})
	}
}

// TestAtomicPolicyAdmissionImportPublishesOriginalWithdrawal observes the
// actual ingress publication, rather than only the policy's boolean verdict.
// A refused marked replacement withdraws its original key; an independent
// previously accepted sibling is not named by that withdrawal.
func TestAtomicPolicyAdmissionImportPublishesOriginalWithdrawal(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	original, sibling := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	var published [][]byte
	f.r.messageReceiver = &testDeliveryReceiver{consumerCount: 1,
		onReceived: func(_ plugin.PeerInfo, update bgptypes.RawMessage) {
			require.NotNil(t, update.WireUpdate)
			published = append(published, bytes.Clone(update.WireUpdate.Payload()))
		}}
	f.r.orderedIngressSteps = []orderedIngressStep{{name: policyChainStepName, policyChain: true}}
	f.source.settings.ImportPolicy = answerPolicy(t, routepolicy.FinalAccept, "admission")
	f.source.refreshForwardFacts()
	f.r.filterTransportSeam = &answerTransport{reads: []string{rpc.FilterFieldRaw},
		answer: func(input rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
			if len(published) < 2 {
				return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
			}
			return rpc.FilterUpdateOutput{Action: rpc.FilterModify,
				Raw: rfc4271ProofRelayBody([]byte{25, 10, 42, 0, 0}, false, 77)}
		}}
	f.receive(t, rfc4271ProofRelayBody(original, true, 10))
	f.receive(t, rfc4271ProofRelayBody(sibling, false, 10))
	f.receive(t, rfc4271ProofRelayBody(original, true, 20))
	require.Len(t, published, 3)
	for index, want := range [][]byte{original, sibling} {
		update, err := message.UnpackUpdate(published[index])
		require.NoError(t, err)
		require.Equal(t, want, update.NLRI)
	}
	update, err := message.UnpackUpdate(published[2])
	require.NoError(t, err)
	require.Equal(t, original, update.WithdrawnRoutes)
	require.Empty(t, update.NLRI)
	require.Empty(t, update.PathAttributes)
}

// TestAtomicPolicyAdmissionExplicitWithdrawalRemainsUseful requires the policy
// to accept a materialized withdrawal and the final writer to remove the prior
// advertisement. Its source marker does not prohibit taking the route back.
func TestAtomicPolicyAdmissionExplicitWithdrawalRemainsUseful(t *testing.T) {
	prefix := []byte{24, 10, 42, 0}
	for _, rs := range []bool{false, true} {
		t.Run(atomicAdmissionRailName(rs), func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(prefix, true, 10), rs)
			withdrawal := makeUpdateBody(prefix, nil, nil)
			exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
				return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: withdrawal}
			}, f.destination)
			input := rfc4271ProofRelayBody(prefix, true, 20)
			step := f.r.runEgressPolicyChain(f.destination.settings.ExportPolicy,
				f.destination.Settings().Address.String(), 65002, 65000, false, explicitExportTestSession(true), wireu.NewWireUpdate(input, f.ctxID))
			require.True(t, step.accept, "withdrawal is useful policy output, not a refusal")
			require.NotNil(t, step.wireOverride)
			require.Equal(t, withdrawal, step.wireOverride.Payload())
			rfc4271AtomicPolicyRelay(t, f, input, rs)
			written := aigpSocketBodies(t, f.conn)
			require.Len(t, written, 2)
			out, err := message.UnpackUpdate(written[1])
			require.NoError(t, err)
			require.Equal(t, prefix, out.WithdrawnRoutes)
			require.Empty(t, out.NLRI)
		})
	}
}

// TestAtomicPolicyAdmissionEORRemainsUseful observes acceptance of an empty
// replacement: unlike a guard refusal, it is an actual wire override with no
// destination, so preserving the marker would create no useful requirement.
func TestAtomicPolicyAdmissionEORRemainsUseful(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	eor := makeUpdateBody(nil, nil, nil)
	exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: eor}
	}, f.destination)
	input := rfc4271ProofRelayBody([]byte{24, 10, 42, 0}, true, 10)
	step := f.r.runEgressPolicyChain(f.destination.settings.ExportPolicy,
		f.destination.Settings().Address.String(), 65002, 65000, false, explicitExportTestSession(true), wireu.NewWireUpdate(input, f.ctxID))
	require.True(t, step.accept)
	require.NotNil(t, step.wireOverride)
	require.Equal(t, eor, step.wireOverride.Payload())
}

// TestAtomicPolicyAdmissionStoredReplayOutlivesCache reconstructs a received
// marked route after the live cache has expired, then asks export to strip its
// marker. The old recipient ownership must leave through the same final writer.
func TestAtomicPolicyAdmissionStoredReplayOutlivesCache(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	prefix := []byte{24, 10, 42, 0}
	body := rfc4271ProofRelayBody(prefix, true, 10)
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	attrs := hex.EncodeToString(sections.Attrs(body))
	id := f.receive(t, body)
	f.forward(t, id)
	f.drain(t)
	require.False(t, f.r.recentUpdates.Contains(id), "the replay may not rely on the transient receipt")
	exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rfc4271ProofRelayBody(prefix, false, 20)}
	}, f.destination)
	stored := rpc.StoredRoute{SourcePeer: f.source.Settings().Address.String(),
		SourceOwner: f.source.sourceOwner, SourceReceipt: f.source.currentSession().sourceReceiptID(),
		SourceConnectionID: f.source.currentSession().connectionID,
		SourcePolicyEpoch:  f.source.sourcePolicyEpoch.Load(), MsgID: id,
		Family: "ipv4/unicast", AttrHex: attrs, NextHopHex: "c612e701",
		NLRIHex: hex.EncodeToString(prefix), NLRIFraming: rpc.NLRIFramingPrefixOnly}
	require.NoError(t, (&reactorAPIAdapter{r: f.r}).RelayStoredRoute(
		f.destination.Settings().Address, []rpc.StoredRoute{stored}, plugin.ProcessSender("aigp-forwarder")))
	f.drain(t)
	written := aigpSocketBodies(t, f.conn)
	require.Len(t, written, 2)
	out, err := message.UnpackUpdate(written[1])
	require.NoError(t, err)
	require.Equal(t, prefix, out.WithdrawnRoutes)
	require.Empty(t, out.NLRI)
}

// TestAtomicPolicyAdmissionAddPathZeroUsesReceivedIdentity proves zero is a
// real received identifier. Refusal withdraws its assigned recipient path;
// changing only the received identifier cannot introduce a new path identity.
func TestAtomicPolicyAdmissionAddPathZeroUsesReceivedIdentity(t *testing.T) {
	prefix := []byte{24, 10, 42, 0}
	for _, rs := range []bool{false, true} {
		for _, changedID := range []bool{false, true} {
			t.Run(atomicAdmissionRailName(rs)+"/"+map[bool]string{false: "strip-marker", true: "new-path-id"}[changedID], func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
				atomicAdmissionContext(t, f, ctx)
				original := append(binary.BigEndian.AppendUint32(nil, 0), prefix...)
				rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(original, true, 10), rs)
				initial := aigpSocketBodies(t, f.conn)
				require.Len(t, initial, 1)
				first, err := message.UnpackUpdate(initial[0])
				require.NoError(t, err)
				require.Equal(t, prefix, first.NLRI[4:])
				output, marker := original, false
				if changedID {
					output, marker = append(binary.BigEndian.AppendUint32(nil, 77), prefix...), true
				}
				exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
					return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rfc4271ProofRelayBody(output, marker, 20)}
				}, f.destination)
				rfc4271AtomicPolicyRelay(t, f, rfc4271ProofRelayBody(original, true, 20), rs)
				written := aigpSocketBodies(t, f.conn)
				require.Len(t, written, 2)
				last, err := message.UnpackUpdate(written[1])
				require.NoError(t, err)
				require.Equal(t, first.NLRI, last.WithdrawnRoutes, "the assigned identifier belongs to the original zero-ID receipt")
				require.Empty(t, last.NLRI)
			})
		}
	}
}

// TestAtomicPolicyAdmissionQueuedSplitSurvives forces useful marked output
// through a held forwarding worker and destination splitting. A blanket guard
// or an alias overwritten after admission loses the exact destination census.
func TestAtomicPolicyAdmissionQueuedSplitSurvives(t *testing.T) {
	entered, release := make(chan struct{}, 1), make(chan struct{})
	f := newAIGPReplayFixture(t, func(items []fwdItem) {
		for i := range items {
			if items[i].peer != nil {
				entered <- struct{}{}
				<-release
				return
			}
		}
	})
	// The received body is legal only under RFC 8654. The destination keeps
	// its ordinary context, requiring real 4096-byte output splitting.
	receive := bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{
		ASN4: true, ExtendedMessageRecv: true,
	}, bgpctx.DirectionRecv)
	receiveID, err := bgpctx.Registry.Register(receive)
	require.NoError(t, err)
	f.ctxID, f.source.recvCtxID = receiveID, receiveID
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	var prefixes []byte
	for index := range 1200 {
		prefixes = append(prefixes, 32, 10, byte(index>>8), byte(index), 1)
	}
	output := rfc4271ProofRelayBody(prefixes, true, 77)
	exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
	}, f.destination)
	id := f.receive(t, rfc4271ProofRelayBody(prefixes, true, 10))
	f.forward(t, id)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("useful marked output did not enter the worker")
	}
	close(release)
	f.drain(t)
	written := aigpSocketBodies(t, f.conn)
	require.Greater(t, len(written), 1, "the destination limit must actually split the accepted policy answer")
	var got []byte
	for _, body := range written {
		require.LessOrEqual(t, message.HeaderLen+len(body), message.MaxMsgLen)
		out, err := message.UnpackUpdate(body)
		require.NoError(t, err)
		got = append(got, out.NLRI...)
		_, _, _, marked := attribute.AttrFind(out.PathAttributes, attribute.AttrAtomicAggregate)
		require.True(t, marked)
		metric := payloadMED(body)
		require.True(t, metric.present)
		require.Equal(t, uint32(77), binary.BigEndian.Uint32(metric.raw))
	}
	require.Equal(t, prefixes, got)
}

// TestAtomicPolicyAdmissionMPUsefulOutput reads exact native IPv6 keys at both
// sockets. Attribute changes, subset removal and reordering remain useful even
// beside a legacy announcement; a blanket refusal cannot satisfy these cases.
func TestAtomicPolicyAdmissionMPUsefulOutput(t *testing.T) {
	first, second := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 1}, []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2}
	input := append(bytes.Clone(first), second...)
	for _, rs := range []bool{false, true} {
		for _, mixed := range []bool{false, true} {
			var legacy []byte
			name := "mp-only"
			if mixed {
				legacy, name = []byte{24, 10, 42, 0}, "legacy-and-mp"
			}
			for _, row := range []struct {
				name string
				nlri []byte
			}{
				{"equal", input}, {"subset", second},
				{"reordered", append(bytes.Clone(second), first...)},
			} {
				t.Run(atomicAdmissionRailName(rs)+"/"+name+"/"+row.name, func(t *testing.T) {
					f := atomicAdmissionMPFixture(t)
					output := atomicAdmissionMPBody(t, legacy, row.nlri, nil, true, 77)
					exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
					}, f.destination)
					rfc4271AtomicPolicyRelay(t, f, atomicAdmissionMPBody(t, legacy, input, nil, true, 10), rs)
					written := aigpSocketBodies(t, f.conn)
					require.NotEmpty(t, written)
					var gotLegacy, gotMP []byte
					for _, body := range written {
						update, err := message.UnpackUpdate(body)
						require.NoError(t, err)
						gotLegacy = append(gotLegacy, update.NLRI...)
						require.Empty(t, update.WithdrawnRoutes)
						_, _, reach, present := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
						if present {
							require.GreaterOrEqual(t, len(reach), 4)
							require.GreaterOrEqual(t, len(reach), 5+int(reach[3]))
							require.Equal(t, []byte{0, 2, 1}, reach[:3])
							gotMP = append(gotMP, reach[5+int(reach[3]):]...)
						}
						_, _, _, marked := attribute.AttrFind(update.PathAttributes, attribute.AttrAtomicAggregate)
						require.True(t, marked)
						metric := payloadMED(body)
						require.True(t, metric.present)
						require.Equal(t, uint32(77), binary.BigEndian.Uint32(metric.raw))
					}
					require.Equal(t, legacy, gotLegacy)
					require.Equal(t, row.nlri, gotMP)
				})
			}
		}
	}
}

// TestAtomicPolicyAdmissionMPRefusalWithdrawsSource observes the old MP keys,
// never the policy's proposed /49. The mixed case carries MP withdrawals of a
// different family, forcing the native rail to hand the whole judgment back to
// cached forwarding rather than invoke the policy again on isolated sections.
func TestAtomicPolicyAdmissionMPRefusalWithdrawsSource(t *testing.T) {
	mp := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 1}
	specific := []byte{49, 0x20, 1, 0x0d, 0xb8, 0, 1, 0}
	for _, rs := range []bool{false, true} {
		for _, mixed := range []bool{false, true} {
			for _, strip := range []bool{false, true} {
				name := "mp-only"
				var legacy, withdrawn []byte
				if mixed {
					name, legacy, withdrawn = "mixed-fallback", []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
				}
				edit := "more-specific"
				outputMP := specific
				if strip {
					edit, outputMP = "strip-marker", mp
				}
				t.Run(atomicAdmissionRailName(rs)+"/"+name+"/"+edit, func(t *testing.T) {
					f := atomicAdmissionMPFixture(t)
					input := atomicAdmissionMPBody(t, legacy, mp, withdrawn, true, 10)
					rfc4271AtomicPolicyRelay(t, f, input, rs)
					before := len(f.conn.written())
					require.Positive(t, before, "a refusal must retire a real prior announcement")
					output := atomicAdmissionMPBody(t, legacy, outputMP, withdrawn, !strip, 77)
					exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
					}, f.destination)
					atomicAdmissionRelayWithFallback(t, f, input, rs, mixed)
					events := ownershipRailRead(t, f.conn.written()[before:], f.destination.sendCtx.Load())
					want := []ownershipRailEvent{{family: family.IPv6Unicast, key: string(mp), withdraw: true}}
					if mixed {
						want = append(want, ownershipRailEvent{family: family.IPv4Unicast, key: string(legacy), withdraw: true})
					}
					require.ElementsMatch(t, want, events, "only the source's previously advertised native keys may leave")
					transport, ok := f.r.filterTransportSeam.(*answerTransport)
					require.True(t, ok)
					calls := 1
					if rs && mixed {
						calls = 2 // One native whole-message judgment, then the cached whole-message judgment.
					}
					require.Equal(t, calls, transport.calls())
				})
			}
		}
	}
}

// TestAtomicPolicyAdmissionRefusalPreservesOtherOwner first gives the key to a
// foreign source or a local origin. The refusing received source cannot take
// either back. A later authorized removal proves the owner survived the refusal.
func TestAtomicPolicyAdmissionRefusalPreservesOtherOwner(t *testing.T) {
	prefix := []byte{24, 10, 42, 0}
	for _, rs := range []bool{false, true} {
		for _, local := range []bool{false, true} {
			name := "foreign-source"
			if local {
				name = "local-origin"
			}
			t.Run(atomicAdmissionRailName(rs)+"/"+name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				f.destination.session.adjOut = &f.destination.adjOut
				foreign, _ := newAnnouncePeer(t, "198.18.233.3")
				foreign.recvCtxID = f.ctxID
				body := rfc4271ProofRelayBody(prefix, true, 10)
				if local {
					require.NoError(t, f.destination.session.SendUpdate(ownershipWriterUpdate(t, body)))
				} else {
					require.NoError(t, ownershipWriterForward(t, f.destination, foreign, body, false))
				}
				require.Len(t, aigpSocketBodies(t, f.conn), 1)
				exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
					return rpc.FilterUpdateOutput{Action: rpc.FilterModify,
						Raw: rfc4271ProofRelayBody([]byte{25, 10, 42, 0, 0}, false, 77)}
				}, f.destination)
				rfc4271AtomicPolicyRelay(t, f, body, rs)
				require.Len(t, aigpSocketBodies(t, f.conn), 1, "refusal cannot withdraw another owner")
				f.destination.settings.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
				f.destination.refreshForwardFacts()
				withdrawal := makeUpdateBody(prefix, nil, nil)
				if local {
					require.NoError(t, f.destination.session.SendUpdate(ownershipWriterUpdate(t, withdrawal)))
				} else {
					require.NoError(t, ownershipWriterForward(t, f.destination, foreign, withdrawal, false))
				}
				written := aigpSocketBodies(t, f.conn)
				require.Len(t, written, 2, "the surviving owner can still remove its route")
				last, err := message.UnpackUpdate(written[1])
				require.NoError(t, err)
				require.Equal(t, prefix, last.WithdrawnRoutes)
				require.Empty(t, last.NLRI)
			})
		}
	}
}

// TestAtomicPolicyAdmissionTypedSubsetRemainsUseful runs the existing typed
// drop and strip-family intents through both socket rails. The original marked
// destinations may be removed without suppressing the remaining family or key.
func TestAtomicPolicyAdmissionTypedSubsetRemainsUseful(t *testing.T) {
	first, second := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	mpFirst, mpSecond := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 1}, []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2}
	for _, rs := range []bool{false, true} {
		for _, strip := range []bool{false, true} {
			name := "drop-selected-keys"
			if strip {
				name = "strip-mp-family"
			}
			t.Run(atomicAdmissionRailName(rs)+"/"+name, func(t *testing.T) {
				f := atomicAdmissionMPFixture(t)
				libraryIntentEval = dropAt(nlriDrop{policychain.NLRISectionAnnounce, 0}, nlriDrop{policychain.NLRISectionMPReach, 0})
				want := []ownershipRailEvent{
					{family: family.IPv4Unicast, key: string(second)},
					{family: family.IPv6Unicast, key: string(mpSecond)},
				}
				if strip {
					libraryIntentEval = func(_ *policyview.View, sets *policychain.Sets) error {
						return sets.StripFamily(family.IPv6Unicast)
					}
					want = []ownershipRailEvent{
						{family: family.IPv4Unicast, key: string(first)},
						{family: family.IPv4Unicast, key: string(second)},
					}
				}
				f.r.orderedEgressSteps = buildOrderedEgressSteps()
				f.destination.settings.ExportPolicy = libraryPolicy(t, libraryIntentExport, libraryTypeIntents, routepolicy.FinalAccept)
				f.destination.refreshForwardFacts()
				input := atomicAdmissionMPBody(t, append(bytes.Clone(first), second...), append(bytes.Clone(mpFirst), mpSecond...), nil, true, 10)
				rfc4271AtomicPolicyRelay(t, f, input, rs)
				events := ownershipRailRead(t, f.conn.written(), f.destination.sendCtx.Load())
				require.ElementsMatch(t, want, events)
				for _, body := range aigpSocketBodies(t, f.conn) {
					out, err := message.UnpackUpdate(body)
					require.NoError(t, err)
					_, _, _, marked := attribute.AttrFind(out.PathAttributes, attribute.AttrAtomicAggregate)
					require.True(t, marked)
				}
			})
		}
	}
}

// TestAtomicPolicyAdmissionFinalJudgeReachesSocket runs a real two-pass export
// chain on both rails. A marked route keeps its exact key, while an unmarked
// original may gain both a new /25 and the marker before the typed final judge.
func TestAtomicPolicyAdmissionFinalJudgeReachesSocket(t *testing.T) {
	for _, rs := range []bool{false, true} {
		for _, marked := range []bool{false, true} {
			name := "unmarked-original-adds-marker"
			prefix := []byte{25, 10, 42, 0, 0}
			if marked {
				name, prefix = "marked-original-keeps-key", []byte{24, 10, 42, 0}
			}
			t.Run(atomicAdmissionRailName(rs)+"/"+name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				input := rfc4271ProofRelayBody([]byte{24, 10, 42, 0}, marked, 10)
				transport := &atomicAdmissionFinalJudge{answerTransport: &answerTransport{
					reads: []string{rpc.FilterFieldRaw}, sets: []string{"med"},
				}}
				transport.answer = func(route rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
					require.Equal(t, input, route.Raw)
					if transport.calls() == 1 {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rfc4271ProofRelayBody(prefix, true, 20)}
					}
					return answerSet("med", "77")
				}
				f.r.filterTransportSeam = transport
				f.r.orderedEgressSteps = buildOrderedEgressSteps()
				f.destination.settings.ExportPolicy = answerPolicy(t, routepolicy.FinalAccept, "judge")
				f.destination.refreshForwardFacts()
				rfc4271AtomicPolicyRelay(t, f, input, rs)
				require.Equal(t, 2, transport.calls(), "the useful replacement must reach the final judge")
				written := aigpSocketBodies(t, f.conn)
				require.Len(t, written, 1)
				out, err := message.UnpackUpdate(written[0])
				require.NoError(t, err)
				require.Equal(t, prefix, out.NLRI)
				require.Empty(t, out.WithdrawnRoutes)
				_, _, _, marker := attribute.AttrFind(out.PathAttributes, attribute.AttrAtomicAggregate)
				require.True(t, marker)
				metric := payloadMED(written[0])
				require.True(t, metric.present)
				require.Equal(t, uint32(77), binary.BigEndian.Uint32(metric.raw))
			})
		}
	}
}

// TestAtomicPolicyAdmissionFinalJudgeUsesOriginalInput isolates the final-pass
// boundary with a typed MED judge and an already rewritten body. Removing the
// intermediate marker must not erase the original condition; the identical
// unmarked original still permits the rewrite. No raw final answer is used:
// that separate contract refuses raw replacement after any body rewrite.
func TestAtomicPolicyAdmissionFinalJudgeUsesOriginalInput(t *testing.T) {
	for _, marked := range []bool{false, true} {
		name := "unmarked-original"
		if marked {
			name = "marked-original"
		}
		t.Run(name, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			original := rfc4271ProofRelayBody([]byte{24, 10, 42, 0}, marked, 10)
			intermediate := rfc4271ProofRelayBody([]byte{25, 10, 42, 0, 0}, false, 20)
			transport := &atomicAdmissionFinalJudge{answerTransport: &answerTransport{
				reads: []string{rpc.FilterFieldRaw}, sets: []string{"med"},
				answer: func(input rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
					require.Equal(t, original, input.Raw, "a final judge reads the original chain input")
					return answerSet("med", "77")
				}}}
			f.r.filterTransportSeam = transport
			policy := answerPolicy(t, routepolicy.FinalAccept, "judge")
			seed := libraryResult{outcome: policychain.Outcome{Action: policychain.OutcomeAccept}, modifiedPayload: intermediate}
			result := f.r.runFinalJudges(policy.Library(), seed, original, f.ctxID, explicitExportTestSession(true),
				libraryIntentContext{asn4: true, asPathEditable: true}, "atomic-final-test", "198.18.232.2", nil)
			require.Equal(t, 1, transport.calls())
			require.False(t, result.failed)
			if marked {
				require.Equal(t, policychain.OutcomeWithdraw, result.outcome.Action)
				require.Nil(t, result.modifiedPayload)
				return
			}
			require.Equal(t, policychain.OutcomeAccept, result.outcome.Action)
			update, err := message.UnpackUpdate(result.modifiedPayload)
			require.NoError(t, err)
			require.Equal(t, []byte{25, 10, 42, 0, 0}, update.NLRI)
			metric := payloadMED(result.modifiedPayload)
			require.True(t, metric.present)
			require.Equal(t, uint32(77), binary.BigEndian.Uint32(metric.raw))
		})
	}
}

// TestAtomicPolicyAdmissionRawSubsetRetiresRemovedSource advertises both source
// keys before applying the raw subset. Removing A must withdraw its old recipient
// route while B remains useful. The empty subset is an EOR answer, which cannot
// leave either old source route behind merely because it has no announcements.
func TestAtomicPolicyAdmissionRawSubsetRetiresRemovedSource(t *testing.T) {
	first, second := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	original := append(bytes.Clone(first), second...)
	for _, rs := range []bool{false, true} {
		for _, empty := range []bool{false, true} {
			name := "raw-subset"
			kept, removed := second, first
			if empty {
				name, kept, removed = "raw-empty-subset-eor", nil, original
			}
			t.Run(atomicAdmissionRailName(rs)+"/"+name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				input := rfc4271ProofRelayBody(original, true, 10)
				rfc4271ProofRelay(t, f, input, rs)
				before := len(aigpSocketBodies(t, f.conn))
				require.Equal(t, 1, before)
				output := rfc4271ProofRelayBody(kept, true, 77)
				if empty {
					output = makeUpdateBody(nil, nil, nil)
				}
				exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
					return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
				}, f.destination)
				rfc4271AtomicPolicyRelay(t, f, input, rs)
				written := aigpSocketBodies(t, f.conn)
				atomicAdmissionAssertRetirement(t, written[before:], kept, removed)
			})
		}
	}
}

// TestAtomicPolicyAdmissionImportRawSubsetRetiresRemovedSource observes the
// real ingress publication after A and B were previously accepted. An import
// subset must publish A's removal to its receive-store consumers, not only the
// useful B answer; the daemon fixture checks the resulting actual Adj-RIB-In.
func TestAtomicPolicyAdmissionImportRawSubsetRetiresRemovedSource(t *testing.T) {
	first, second := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	original := append(bytes.Clone(first), second...)
	for _, empty := range []bool{false, true} {
		name := "raw-subset"
		kept, removed := second, first
		if empty {
			name, kept, removed = "raw-empty-subset-eor", nil, original
		}
		t.Run(name, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			var published [][]byte
			f.r.messageReceiver = &testDeliveryReceiver{consumerCount: 1,
				onReceived: func(_ plugin.PeerInfo, update bgptypes.RawMessage) {
					require.NotNil(t, update.WireUpdate)
					published = append(published, bytes.Clone(update.WireUpdate.Payload()))
				}}
			input := rfc4271ProofRelayBody(original, true, 10)
			f.receive(t, input)
			require.Len(t, published, 1)
			f.r.orderedIngressSteps = []orderedIngressStep{{name: policyChainStepName, policyChain: true}}
			f.source.settings.ImportPolicy = answerPolicy(t, routepolicy.FinalAccept, "admission")
			f.source.refreshForwardFacts()
			output := rfc4271ProofRelayBody(kept, true, 77)
			if empty {
				output = makeUpdateBody(nil, nil, nil)
			}
			f.r.filterTransportSeam = rawAnswerTransport(output)
			f.receive(t, input)
			atomicAdmissionAssertRetirement(t, published[1:], kept, removed)
		})
	}
}

// atomicAdmissionAssertRetirement permits merged or separate withdrawal and
// announcement messages. It checks source removals and useful survivor bytes,
// so neither a stale old route nor a blanket source withdrawal can pass.
func atomicAdmissionAssertRetirement(t *testing.T, bodies [][]byte, kept, removed []byte) {
	t.Helper()
	var announced, withdrawn []byte
	for _, body := range bodies {
		update, err := message.UnpackUpdate(body)
		require.NoError(t, err)
		announced = append(announced, update.NLRI...)
		withdrawn = append(withdrawn, update.WithdrawnRoutes...)
		if len(update.NLRI) > 0 {
			_, _, _, marked := attribute.AttrFind(update.PathAttributes, attribute.AttrAtomicAggregate)
			require.True(t, marked)
			metric := payloadMED(body)
			require.True(t, metric.present)
			require.Equal(t, uint32(77), binary.BigEndian.Uint32(metric.raw))
		}
	}
	require.Equal(t, removed, withdrawn, "a subset must retire every source key it removed")
	require.Equal(t, kept, announced, "the exact retained subset must remain useful")
}

// atomicAdmissionFinalJudge changes only the declaration: the existing typed
// answer transport still owns all input construction and callback recording.
type atomicAdmissionFinalJudge struct{ *answerTransport }

func (a *atomicAdmissionFinalJudge) FilterJudgesFinal(pluginName, _ string) (bool, bool) {
	return true, pluginName == answerPlugin
}

func atomicAdmissionMPFixture(t *testing.T) *aigpReplayFixture {
	t.Helper()
	f := newAIGPReplayFixture(t, nil)
	for _, peer := range []*Peer{f.source, f.destination} {
		negotiateForwardTestFamilies(peer, family.IPv4Unicast, family.IPv6Unicast)
		peer.settings.NextHopMode = NextHopUnchanged
		peer.refreshForwardFacts()
	}
	return f
}

func atomicAdmissionMPBody(t *testing.T, legacy, native, withdrawn []byte, marked bool, med byte) []byte {
	t.Helper()
	base := rfc4271ProofRelayBody(legacy, marked, med)
	update, err := message.UnpackUpdate(base)
	require.NoError(t, err)
	attrs := bytes.Clone(update.PathAttributes)
	if native != nil {
		nextHop := []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
		attrs = append(attrs, buildMPReachSource(2, 1, nextHop, native)...)
	}
	if withdrawn != nil {
		attrs = append(attrs, makeAttr(0x80, byte(attribute.AttrMPUnreachNLRI), append([]byte{0, 1, 1}, withdrawn...))...)
	}
	return makeUpdateBody(nil, attrs, legacy)
}

func atomicAdmissionRelayWithFallback(t *testing.T, f *aigpReplayFixture, body []byte, rs, fallback bool) {
	t.Helper()
	if !rs {
		rfc4271AtomicPolicyRelay(t, f, body, false)
		return
	}
	id := f.receive(t, body)
	update, present := f.r.recentUpdates.Get(id)
	require.True(t, present)
	skipped, _ := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
	if fallback {
		require.Equal(t, []rpc.ForwardTarget{capturedForwardTarget(f.destination, f.destination.currentSession())}, skipped)
		require.Empty(t, f.source.session.fwdDirty, "the native rail must not also write this destination")
		f.forward(t, id)
	} else {
		require.Empty(t, skipped)
		f.source.session.flushFwdDirty()
	}
	f.drain(t)
}

func atomicAdmissionRailName(rs bool) string {
	if rs {
		return "native-rs"
	}
	return "cached"
}

func atomicAdmissionContext(t *testing.T, f *aigpReplayFixture, ctx *bgpctx.EncodingContext) {
	t.Helper()
	id, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	f.ctxID, f.source.recvCtxID = id, id
	f.destination.sendCtx.Store(ctx)
	f.destination.sendCtxID, f.destination.session.sendCtxID = id, id
	f.destination.refreshForwardFacts()
}
