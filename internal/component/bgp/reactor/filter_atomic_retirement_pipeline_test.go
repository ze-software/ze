// Design: docs/architecture/bgp/egress-attribute-rules.md -- accepted subsets retire source routes.
// Related: rfc4271_atomic_policy_admission_test.go -- shared live-writer fixtures and original regressions.
package reactor

import (
	"bytes"
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAtomicRetirementFinalPass reads both socket rails after the final judge:
// an unchanged pass keeps the first removal, while a further typed prefix drop
// retires both excluded source keys. Raw rewrites after a prior rewrite remain
// forbidden by policychain.Chain.run; no impossible raw restoration is assumed.
func TestAtomicRetirementFinalPass(t *testing.T) {
	first, second, third := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}, []byte{24, 10, 44, 0}
	original := append(append(bytes.Clone(first), second...), third...)
	for _, rs := range []bool{false, true} {
		for _, dropAgain := range []bool{false, true} {
			name := "unchanged-final-pass"
			kept, removed := append(bytes.Clone(second), third...), first
			if dropAgain {
				name, kept = "additional-typed-drop", third
				removed = append(bytes.Clone(first), second...)
			}
			t.Run(atomicAdmissionRailName(rs)+"/"+name, func(t *testing.T) {
				f := newAIGPReplayFixture(t, nil)
				input := rfc4271ProofRelayBody(original, true, 10)
				rfc4271ProofRelay(t, f, input, rs)
				before := len(aigpSocketBodies(t, f.conn))
				transport := &atomicAdmissionFinalJudge{answerTransport: &answerTransport{
					reads: []string{rpc.FilterFieldRaw, "prefix"}, sets: []string{"prefix"},
				}}
				transport.answer = func(route rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
					require.Equal(t, input, route.Raw, "the final pass retains the original source basis")
					if transport.calls() == 1 {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify,
							Raw: rfc4271ProofRelayBody(append(bytes.Clone(second), third...), true, 77)}
					}
					if dropAgain {
						return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
							intentSet("prefix", rpc.FilterSetActionRemove, `"10.43.0.0/24"`),
						}}
					}
					return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
				}
				f.r.filterTransportSeam = transport
				f.r.orderedEgressSteps = buildOrderedEgressSteps()
				f.destination.settings.ExportPolicy = answerPolicyAt(t, libraryIntentExport, routepolicy.FinalAccept, "judge")
				f.destination.refreshForwardFacts()
				rfc4271AtomicPolicyRelay(t, f, input, rs)
				require.Equal(t, 2, transport.calls())
				atomicAdmissionAssertRetirement(t, aigpSocketBodies(t, f.conn)[before:], kept, removed)
			})
		}
	}
}

// TestAtomicRetirementOriginOwnership goes through the session's real export
// gate. An omitted local A is withdrawn, but the same policy cannot withdraw A
// held for a received source. The retained local B must still leave with MED77.
func TestAtomicRetirementOriginOwnership(t *testing.T) {
	first, second := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	original := append(bytes.Clone(first), second...)
	for _, foreign := range []bool{false, true} {
		name := "local-source"
		if foreign {
			name = "foreign-source"
		}
		t.Run(name, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			f.destination.session.adjOut = &f.destination.adjOut
			f.destination.reactor = f.r
			input := rfc4271ProofRelayBody(original, true, 10)
			if foreign {
				require.NoError(t, f.destination.session.SendUpdate(ownershipWriterUpdate(t, rfc4271ProofRelayBody(second, true, 10))))
				require.NoError(t, ownershipWriterForward(t, f.destination, f.source, rfc4271ProofRelayBody(first, true, 10), false))
			} else {
				require.NoError(t, f.destination.session.SendUpdate(ownershipWriterUpdate(t, input)))
			}
			before := len(aigpSocketBodies(t, f.conn))
			exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
				return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rfc4271ProofRelayBody(second, true, 77)}
			}, f.destination)
			f.destination.session.egressRouteFilter = f.destination.egressGate(f.destination.session)
			require.NoError(t, f.destination.session.SendUpdate(ownershipWriterUpdate(t, input)))
			removed := first
			if foreign {
				removed = nil
			}
			atomicAdmissionAssertRetirement(t, aigpSocketBodies(t, f.conn)[before:], second, removed)
			if !foreign {
				return
			}
			before = len(aigpSocketBodies(t, f.conn))
			require.NoError(t, ownershipWriterForward(t, f.destination, f.source, makeUpdateBody(first, nil, nil), false))
			atomicAdmissionAssertRetirement(t, aigpSocketBodies(t, f.conn)[before:], nil, first)
		})
	}
}

// TestAtomicRetirementImportPublication drives one real notification decision.
// Its derived bodies retain ingress metadata and admission epoch, have increasing
// IDs, and transfer the original pooled receipt only to the final continuation.
// A synthetic EOR must not count as a second physical UPDATE or received EOR.
func TestAtomicRetirementImportPublication(t *testing.T) {
	first, second := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	original := append(bytes.Clone(first), second...)
	for _, empty := range []bool{false, true} {
		name := "kept-subset"
		if empty {
			name = "synthetic-eor"
		}
		t.Run(name, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			f.source.sourcePolicyEpoch.Store(41)
			var published []bgptypes.RawMessage
			var buffers []BufHandle
			var bodies [][]byte
			f.r.messageReceiver = &testDeliveryReceiver{consumerCount: 1,
				onReceived: func(_ plugin.PeerInfo, update bgptypes.RawMessage) {
					published = append(published, update)
					bodies = append(bodies, bytes.Clone(update.WireUpdate.Payload()))
					cached, present := f.r.recentUpdates.Get(update.MessageID)
					require.True(t, present, "publication must follow its cache insertion")
					buffers = append(buffers, cached.poolBuf)
					f.ids = append(f.ids, update.MessageID)
				}}
			annotations := 0
			f.r.orderedIngressSteps = []orderedIngressStep{
				{name: "atomic-test-metadata", inproc: func(_ filterapi.PeerFilterInfo, _ []byte, meta map[string]any) (bool, []byte) {
					annotations++
					meta["atomic-proof"] = "original-admission"
					return true, nil
				}},
				{name: policyChainStepName, policyChain: true},
			}
			f.source.settings.ImportPolicy = answerPolicy(t, routepolicy.FinalAccept, "admission")
			f.source.refreshForwardFacts()
			output := rfc4271ProofRelayBody(second, true, 77)
			kept, removed := second, first
			if empty {
				output, kept, removed = makeUpdateBody(nil, nil, nil), nil, original
			}
			transport := &answerTransport{reads: []string{rpc.FilterFieldRaw}, answer: func(rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
				f.source.sourcePolicyEpoch.Store(42)
				return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
			}}
			f.r.filterTransportSeam = transport
			input := rfc4271ProofRelayBody(original, true, 10)
			buffer := f.source.session.getReadBuffer()
			require.NotNil(t, buffer.Buf)
			adopted := false
			t.Cleanup(func() {
				if !adopted {
					returnReadBuffer(buffer)
				}
			})
			copy(buffer.Buf, input)
			body := buffer.Buf[:len(input)]
			update := wireu.NewWireUpdate(body, f.ctxID)
			update.SetSourceID(f.source.SourceID())
			before := f.source.Stats()
			adopted = f.r.notifyMessageReceiver(f.source.Settings().Address, msgtype.TypeUPDATE, body, update, f.ctxID,
				rpc.DirectionReceived, buffer, nil, "", 0)
			require.True(t, adopted, "the effective continuation must take the original receipt exactly once")
			require.Equal(t, 1, annotations, "continuations must not run ingress again")
			require.Equal(t, 1, transport.calls())
			require.Len(t, published, 2)
			require.Less(t, published[0].MessageID, published[1].MessageID)
			for _, got := range published {
				require.Equal(t, uint64(41), got.SourcePolicyEpoch, "a delayed answer cannot acquire the new epoch")
				require.Equal(t, "original-admission", got.Meta["atomic-proof"])
				require.Equal(t, f.source.SourceID(), got.WireUpdate.SourceID())
			}
			require.Equal(t, noPoolBufID, buffers[0].ID, "removals own heap bytes, not the original pool slot")
			require.Equal(t, buffer.ID, buffers[1].ID)
			require.Equal(t, buffer.idx, buffers[1].idx)
			require.Same(t, &buffer.Buf[0], &buffers[1].Buf[0])
			after := f.source.Stats()
			require.Equal(t, before.UpdatesReceived+1, after.UpdatesReceived)
			require.Equal(t, before.EORReceived, after.EORReceived)
			atomicAdmissionAssertRetirement(t, bodies, kept, removed)
		})
	}
}

// TestAtomicRetirementQueuedGroup holds the destination writer while either
// forwarding rail dispatches. One source decision must remain one queued item
// and one native destination count, with its removals ahead of useful B or EOR.
func TestAtomicRetirementQueuedGroup(t *testing.T) {
	first, second := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	original := append(bytes.Clone(first), second...)
	for _, rs := range []bool{false, true} {
		for _, empty := range []bool{false, true} {
			name := "kept-subset"
			if empty {
				name = "empty-subset-eor"
			}
			t.Run(atomicAdmissionRailName(rs)+"/"+name, func(t *testing.T) {
				var queued atomic.Int32
				f := newAIGPReplayFixture(t, func(items []fwdItem) {
					for i := range items {
						if items[i].peer != nil {
							queued.Add(1)
						}
					}
				})
				input := rfc4271ProofRelayBody(original, true, 10)
				rfc4271ProofRelay(t, f, input, rs)
				before := len(aigpSocketBodies(t, f.conn))
				queuedBefore := queued.Load()
				output := rfc4271ProofRelayBody(second, true, 77)
				kept, removed := second, first
				if empty {
					output, kept, removed = makeUpdateBody(nil, nil, nil), nil, original
				}
				exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
					return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
				}, f.destination)
				id := f.receive(t, input)
				func() {
					f.destination.session.writeMu.Lock()
					defer f.destination.session.writeMu.Unlock()
					if rs {
						update, present := f.r.recentUpdates.Get(id)
						require.True(t, present)
						f.r.rsForwardingEnabled = true
						defer func() { f.r.rsForwardingEnabled = false }()
						skipped, destinations := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
						require.Empty(t, skipped)
						require.Equal(t, 1, destinations, "removal bodies are not additional destinations")
						return
					}
					f.forward(t, id)
				}()
				f.source.session.flushFwdDirty()
				f.drain(t)
				require.Equal(t, queuedBefore+1, queued.Load(), "the removal and kept body share one queue item")
				written := aigpSocketBodies(t, f.conn)[before:]
				atomicAdmissionAssertRetirement(t, written, kept, removed)
				require.GreaterOrEqual(t, len(written), 2)
				removal, err := message.UnpackUpdate(written[0])
				require.NoError(t, err)
				require.Equal(t, removed, removal.WithdrawnRoutes, "retirement must precede the kept output")
				require.Empty(t, removal.NLRI)
				if empty {
					_, endOfRIB := wireu.NewWireUpdate(written[len(written)-1], f.ctxID).IsEOR()
					require.True(t, endOfRIB, "the grouped EOR follows all source withdrawals")
				}
			})
		}
	}
}

// TestAtomicRetirementExpectedReceipt preserves the exact replay fence through
// a provenance-bearing removal. A newer advertisement from the SAME source
// supersedes old A, while B still names the old receipt and remains useful.
func TestAtomicRetirementExpectedReceipt(t *testing.T) {
	first, second := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	original := append(bytes.Clone(first), second...)
	f := newAIGPReplayFixture(t, nil)
	f.destination.session.adjOut = &f.destination.adjOut
	adjOutTestForward(t, f.destination, f.source, original, 11)
	previous := f.destination.session.sentReceipt.MessageID()
	adjOutTestForward(t, f.destination, f.source, first, 12)
	require.Greater(t, f.destination.session.sentReceipt.MessageID(), previous)
	before := len(aigpSocketBodies(t, f.conn))
	input, output := rfc4271ProofRelayBody(original, true, 10), rfc4271ProofRelayBody(second, true, 77)
	exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
	}, f.destination)
	id := f.receive(t, input)
	received, present := f.r.recentUpdates.Get(id)
	require.True(t, present)
	result := f.r.runEgressPolicyChainASN4(f.destination.settings.ExportPolicy, f.destination.addrString,
		f.destination.PeerAS(), f.destination.settings.LocalAS, false,
		explicitExportTestSession(true), received.WireUpdate, true)
	require.True(t, result.accept)
	require.NotNil(t, result.wireOverride)
	item := fwdItem{peer: f.destination, session: f.destination.session, rawBodies: [][]byte{result.wireOverride.Payload()},
		authority: adjOutExpected, expectedMessage: previous, receivedPeer: f.source,
		receivedGeneration: f.source.forwardGeneration.Load(), sourcePolicyEpoch: f.source.sourcePolicyEpoch.Load(), sourceMessageID: 11}
	defer func() {
		if item.provenance != nil {
			item.provenance.release()
		}
		for i := range item.policyRemovals {
			if item.policyRemovals[i].provenance != nil {
				item.policyRemovals[i].provenance.release()
			}
		}
	}()
	require.NoError(t, prepareFwdPolicy(&item, received, received.WireUpdate, result.wireOverride, false, result.removals,
		f.destination.forwardFacts()))
	require.Len(t, item.policyRemovals, 1)
	require.NotNil(t, item.policyRemovals[0].provenance, "the regression must exercise a manifested child")
	session := f.destination.session
	func() {
		session.writeMu.Lock()
		defer session.writeMu.Unlock()
		session.sentForward = &item
		defer func() { session.sentForward = nil }()
		require.NoError(t, session.writePolicyRemovals(&item))
		require.NoError(t, session.writeRawUpdateBody(result.wireOverride.Payload()))
		require.NoError(t, session.flushWrites())
	}()
	atomicAdmissionAssertRetirement(t, aigpSocketBodies(t, f.conn)[before:], second, nil)
	before = len(aigpSocketBodies(t, f.conn))
	withdrawal := fwdItem{receivedPeer: f.source, receivedGeneration: f.source.forwardGeneration.Load(),
		sourcePolicyEpoch: f.source.sourcePolicyEpoch.Load(), authority: adjOutForwarded, sourceMessageID: 13}
	adjOutTestSend(t, f.destination, f.source, &withdrawal, makeUpdateBody(first, nil, nil))
	atomicAdmissionAssertRetirement(t, aigpSocketBodies(t, f.conn)[before:], nil, first)
}

// TestAtomicRetirementStaleSender drives the actual stale-unit sender with a
// prior sent receipt. The new policy leaves B, so its A withdrawal must reach
// the final writer even when the opted-in second pass changes nothing.
func TestAtomicRetirementStaleSender(t *testing.T) {
	first, second := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	f := newAIGPReplayFixture(t, nil)
	f.destination.session.adjOut = &f.destination.adjOut
	f.destination.reactor = f.r
	batch := staleReadvertiseBatch(t)
	batch.NLRIs = nil
	for _, raw := range [][]byte{first, second} {
		prefix, err := nlri.NewWireNLRI(family.IPv4Unicast, raw, false)
		require.NoError(t, err)
		batch.NLRIs = append(batch.NLRIs, prefix)
	}
	batch.Attrs.SetAtomicAggregate().SetMED(10)
	batch.Replay = true
	adapter := &reactorAPIAdapter{r: f.r}
	facts := announceFactsFor(f.destination, batch.Family, netip.MustParseAddr("10.0.0.1"), false, f.destination.negotiated.Load())
	seed, err := adapter.buildBatchAnnounceUpdate(make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen), batch, facts)
	require.NoError(t, err)
	require.NoError(t, f.destination.session.SendUpdate(seed))
	batch.SentOwnerMessage = f.destination.session.sentReceipt.MessageID()
	require.NotZero(t, batch.SentOwnerMessage)
	before := len(aigpSocketBodies(t, f.conn))
	exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rfc4271ProofRelayBody(second, true, 77)}
	}, f.destination)
	f.destination.raiseExportPolicyFrom(batch.SentOwnerMessage + 1)
	sent, err := adapter.sendStaleReadvertiseUnit(t.Context(), announceTarget{peer: f.destination, session: f.destination.session},
		batch, facts, message.MaxMsgLen, make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen))
	require.NoError(t, err)
	require.True(t, sent)
	atomicAdmissionAssertRetirement(t, aigpSocketBodies(t, f.conn)[before:], second, first)
}

// TestAtomicRetirementOriginFailureCommitsRemoval refuses unusable B after
// accepting the subset. Own A must already be withdrawn on the real socket,
// even when normalization suppresses B or final admission returns an error.
// A received source's A remains protected from this unowned local operation.
func TestAtomicRetirementOriginFailureCommitsRemoval(t *testing.T) {
	first, second := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 1}, []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2}
	global := netip.MustParseAddr("2001:db8::1").AsSlice()
	linkLocal := netip.MustParseAddr("fe80::1").AsSlice()
	for _, normalizerFails := range []bool{false, true} {
		for _, foreign := range []bool{false, true} {
			name := "final-admission"
			if normalizerFails {
				name = "normalizer"
			}
			owner := "local-owner"
			if foreign {
				owner = "foreign-owner"
			}
			t.Run(name+"/"+owner, func(t *testing.T) {
				f := atomicAdmissionMPFixture(t)
				f.destination.session.adjOut = &f.destination.adjOut
				f.destination.reactor = f.r
				seed := atomicRetirementMPNextHopBody(t, first, global, 10)
				if foreign {
					require.NoError(t, ownershipWriterForward(t, f.destination, f.source, seed, false))
				} else {
					require.NoError(t, f.destination.session.SendUpdate(ownershipWriterUpdate(t, seed)))
				}
				before := len(aigpSocketBodies(t, f.conn))
				require.Equal(t, 1, before, "the source A must actually be advertised first")
				nextHop := linkLocal
				if normalizerFails {
					nextHop = append(bytes.Clone(global), linkLocal...)
					// The valid pair must be trimmed off-link. A missing registered
					// rewrite handler exercises the real normalizer failure branch.
					delete(f.r.attrModHandlers, uint8(attribute.AttrMPReachNLRI))
				}
				output := atomicRetirementMPNextHopBody(t, second, nextHop, 77)
				exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
					return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
				}, f.destination)
				f.destination.session.egressRouteFilter = f.destination.egressGate(f.destination.session)
				input := atomicRetirementMPNextHopBody(t, append(bytes.Clone(first), second...), global, 10)
				err := f.destination.session.SendUpdate(ownershipWriterUpdate(t, input))
				if normalizerFails {
					require.NoError(t, err, "normalizer suppression is not a transport failure")
				} else {
					require.ErrorIs(t, err, message.ErrUnicastNextHopUnusable)
				}
				written := aigpSocketBodies(t, f.conn)[before:]
				if foreign {
					require.Empty(t, written, "a failed local subset cannot remove the received owner")
					withdrawal := ownershipRailBody(family.IPv6Unicast, true, first, 0)
					require.NoError(t, ownershipWriterForward(t, f.destination, f.source, withdrawal, false))
					written = aigpSocketBodies(t, f.conn)[before:]
				}
				require.Len(t, written, 1, "the admitted removal must reach the socket without waiting for another send")
				update := wireu.NewWireUpdate(written[0], f.ctxID)
				withdrawn, err := update.MPUnreach()
				require.NoError(t, err)
				require.NotNil(t, withdrawn)
				require.Equal(t, first, withdrawn.WithdrawnBytes())
				announced, err := update.MPReach()
				require.NoError(t, err)
				require.Nil(t, announced, "unusable B must never be advertised")
			})
		}
	}
}

func atomicRetirementMPNextHopBody(t *testing.T, native, nextHop []byte, metric byte) []byte {
	t.Helper()
	base, err := message.UnpackUpdate(rfc4271ProofRelayBody(nil, true, metric))
	require.NoError(t, err)
	attrs := append(bytes.Clone(base.PathAttributes), buildMPReachSource(2, 1, nextHop, native)...)
	return makeUpdateBody(nil, attrs, nil)
}
