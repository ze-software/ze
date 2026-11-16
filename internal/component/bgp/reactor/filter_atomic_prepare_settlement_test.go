// Design: docs/architecture/bgp/structural-forwarding.md -- fatal preparation settles after forwarding locks.
// Related: forward_result.go -- captured-session delivery and retirement decisions.
// Related: aigp_readvertise.go -- the independent replay dispatch-lock owner.
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// atomicPreparationFault receives a legal extended mixed UPDATE while its
// recipient has the standard limit. The source subset owes a real legacy key
// and the original indivisible negotiated BGP-LS withdrawal.
func atomicPreparationFault(t *testing.T, rs bool) (*aigpReplayFixture, uint64) {
	t.Helper()
	f := atomicAdmissionMPFixture(t)
	atomicFailureExtendedReceive(t, f)
	bgpls := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	for _, peer := range []*Peer{f.source, f.destination} {
		negotiateForwardTestFamilies(peer, family.IPv4Unicast, family.IPv6Unicast, bgpls)
		peer.refreshForwardFacts()
	}
	f.source.session.negotiated.ExtendedMessageRecv = true
	f.source.negotiated.Store(NewNegotiatedCapabilities(f.source.session.negotiated))
	f.destination.session.adjOut = &f.destination.adjOut
	prior := atomicAdmissionMPBody(t, []byte{24, 10, 42, 0}, []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2}, nil, true, 10)
	rfc4271AtomicPolicyRelay(t, f, prior, rs)
	require.NotEmpty(t, aigpSocketBodies(t, f.conn))
	base, err := message.UnpackUpdate(prior)
	require.NoError(t, err)
	ls := atomicFailureLinkStateNode(5000)
	attrs := append(bytes.Clone(base.PathAttributes), atomicFailureExtendedAttr(0x80, byte(attribute.AttrMPUnreachNLRI), append([]byte{0x40, 4, 71}, ls...))...)
	input := makeUpdateBody(nil, attrs, base.NLRI)
	require.LessOrEqual(t, len(input)+message.HeaderLen, message.ExtMsgLen)
	output := atomicAdmissionMPBody(t, nil, []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2}, nil, true, 77)
	exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
	}, f.destination)
	id := f.receive(t, input)
	atomicFailureAssertAdmitted(t, f, id, output)
	return f, id
}

// TestAtomicPreparationSettlementUnlocksAndSparesReplacement records actual
// notification callback lock predicates, so a bad inline teardown fails an
// assertion instead of hanging. A replacement installed by that callback must
// keep its connection while only the captured failed connection closes.
func TestAtomicPreparationSettlementUnlocksAndSparesReplacement(t *testing.T) {
	for _, rs := range []bool{false, true} {
		t.Run(atomicAdmissionRailName(rs), func(t *testing.T) {
			f, id := atomicPreparationFault(t, rs)
			if !rs {
				ribevents.RegisterValidationLookup(func(ribevents.ValidationRoute, uint64) bool { return true }, func(ribevents.ValidationRoute) bool { return true })
				t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
			}
			captured := f.destination.currentSession()
			replacement, replacementConn := newAnnouncePeer(t, f.destination.Settings().Address.String())
			called, dispatchFree, validationFree := false, false, false
			captured.beforeNotification = func() {
				called = true
				dispatchFree = f.r.aigp.dispatch.TryLock()
				if dispatchFree {
					f.r.aigp.dispatch.Unlock()
				}
				validationFree = f.source.validationForwardMu.TryLock()
				if validationFree {
					f.source.validationForwardMu.Unlock()
				}
				f.destination.mu.Lock()
				f.destination.session = replacement.session
				f.destination.mu.Unlock()
			}
			atomicFailureDispatch(t, f, id, rs, true)
			require.True(t, called, "the actual terminal writer must reach the notification callback")
			require.True(t, dispatchFree, "notification callbacks must run after the AIGP dispatch unlock")
			require.True(t, validationFree, "validation-enabled cached forwarding must release its source mutex")
			require.True(t, captured.tearingDown.Load())
			require.Nil(t, captured.Conn())
			require.Same(t, replacement.session, f.destination.currentSession())
			require.False(t, replacement.session.tearingDown.Load(), "a terminal decision belongs to the captured epoch")
			require.Same(t, replacementConn, replacement.session.Conn())
			require.Empty(t, replacementConn.written())
		})
	}
}

// TestAtomicPreparationInheritedFatalPreservesOrdinaryFailure distinguishes an
// existing fatal capture from a separate destination that cannot forward.
func TestAtomicPreparationInheritedFatalPreservesOrdinaryFailure(t *testing.T) {
	f, id := atomicPreparationFault(t, false)
	update, present := f.r.recentUpdates.Get(id)
	require.True(t, present)
	failed := f.destination.currentSession()
	unavailable, _ := newAnnouncePeer(t, "198.18.234.3")
	unavailable.fwdFacts.Store(nil)
	result := (&reactorAPIAdapter{r: f.r}).forwardUpdateSection(update, id, []*Peer{f.destination, unavailable},
		forwardSourceInfo{resolved: true, peer: f.source}, update.WireUpdate, map[*Session]error{failed: errPolicyRemovalEncoding})
	require.ErrorIs(t, result.err, errNoEstablishedPeersToForwardTo, "a prior fatal capture must not hide an ordinary failed destination")
}

// TestAtomicPreparationFatalOverridesEarlierDelivery keeps terminal captures
// out of final success, including receipts beyond the inline four slots.
func TestAtomicPreparationFatalOverridesEarlierDelivery(t *testing.T) {
	var result forwardResult
	result.trackDelivery = true
	var sessions []*Session
	for i := range 6 {
		peer, _ := newAnnouncePeer(t, "198.18.234.3")
		sessions = append(sessions, peer.session)
		part := forwardResult{trackDelivery: true}
		part.sentTo(peer.session)
		result.merge(part)
		require.Equal(t, i+1, result.deliveries())
	}
	for _, session := range sessions {
		result.fail(session, errPolicyRemovalEncoding)
	}
	require.False(t, result.delivered(), "closing all captured recipients leaves no successful destination")
	require.ErrorIs(t, result.outcome(), errNoEstablishedPeersToForwardTo)
	healthy, _ := newAnnouncePeer(t, "198.18.234.4")
	result.sentTo(healthy.session)
	require.NoError(t, result.outcome(), "a separate healthy recipient preserves partial delivery")
	require.Equal(t, 1, result.deliveries())
}

// TestAtomicAIGPReplayPreparationEncoderBoundFault injects an encoder bound
// into a private destination-facts snapshot after a real ordinary route was
// advertised. The bound is intentionally below the BGP negotiated minimum:
// this proves replay settlement plumbing, not natural RFC-sized failure.
// The real 4096-byte control withdraws the route and keeps the session live.
func TestAtomicAIGPReplayPreparationEncoderBoundFault(t *testing.T) {
	for _, fault := range []bool{false, true} {
		name := "negotiated-bound"
		if fault {
			name = "injected-encoder-bound"
		}
		t.Run(name, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			captured := f.destination.currentSession()
			captured.adjOut = &f.destination.adjOut
			f.metric(7)
			body := f.body(t, 100)
			plain, err := message.UnpackUpdate(body)
			require.NoError(t, err)
			attrs := append(bytes.Clone(plain.PathAttributes), []byte{0x40, byte(attribute.AttrAtomicAggregate), 0}...)
			body = makeUpdateBody(nil, attrs, plain.NLRI)
			f.forward(t, f.receive(t, body))
			f.waitBatch(t)
			seeded := aigpSocketBodies(t, f.conn)
			require.Len(t, seeded, 1)
			metric, present := aigpReceivedMetric(t, seeded[0])
			require.True(t, present)
			require.Equal(t, uint64(107), metric)
			before := len(f.conn.written())
			exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
				return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: makeUpdateBody(nil, nil, nil)}
			}, f.destination)
			originalFacts := f.destination.forwardFacts()
			if fault {
				faultFacts := *originalFacts
				faultFacts.maxMsgSize = message.HeaderLen + 7
				f.destination.fwdFacts.Store(&faultFacts)
				t.Cleanup(func() { f.destination.fwdFacts.Store(originalFacts) })
			}
			called, unlocked := false, false
			captured.beforeNotification = func() {
				called = true
				unlocked = f.r.aigp.dispatch.TryLock()
				if unlocked {
					f.r.aigp.dispatch.Unlock()
				}
			}
			f.metric(11)
			f.r.readvertiseAIGP()
			f.drain(t)
			if fault {
				require.True(t, called, "the actual replay must settle its encoder failure")
				require.True(t, unlocked, "the replay's distinct dispatch owner must unlock before teardown")
				require.True(t, captured.tearingDown.Load())
				require.Nil(t, captured.Conn())
				return
			}
			require.False(t, called)
			require.False(t, captured.tearingDown.Load())
			frames := bgpFrames(t, f.conn.written()[before:])
			var withdrawn []byte
			for _, frame := range frames {
				update, err := message.UnpackUpdate(frame[message.HeaderLen:])
				require.NoError(t, err)
				withdrawn = append(withdrawn, update.WithdrawnRoutes...)
				require.Empty(t, update.NLRI)
			}
			require.Equal(t, plain.NLRI, withdrawn, "the ordinary route is actually removed at the real negotiated bound")
		})
	}
}

// TestAtomicAIGPReplayPrequeueFailureCanRetry proves a failed destination lookup
// leaves the live advertisement undecided and releases its replay claim. A later
// pass at the same metric revision must actually send the recomputed route.
func TestAtomicAIGPReplayPrequeueFailureCanRetry(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.metric(7)
	f.forward(t, f.receive(t, f.body(t, 100)))
	forwardSocketBarrier(t, f.r)
	initial := aigpSocketBodies(t, f.conn)
	require.Len(t, initial, 1)
	lastAnnounces(t, initial, []byte{24, 10, 20, 0}, 107)
	before := f.conn.written()
	facts := f.destination.forwardFacts()
	require.NotNil(t, facts)
	f.destination.fwdFacts.Store(nil)
	t.Cleanup(func() { f.destination.fwdFacts.Store(facts) })
	f.metric(11)
	revision := aigpRevision()
	f.r.readvertiseAIGP()
	forwardSocketBarrier(t, f.r)
	require.Equal(t, before, f.conn.written(), "an unavailable destination must not count as a sent replay")
	f.r.aigp.mu.Lock()
	var queued []aigpOutcome
	var decided []uint64
	for _, ad := range f.r.aigp.advertised {
		queued = append(queued, ad.queued)
		decided = append(decided, ad.revision)
	}
	f.r.aigp.mu.Unlock()
	require.Len(t, queued, 1, "the live recipient remains owed its new metric")
	require.False(t, queued[0].set, "a failure before dispatch must release the in-flight claim")
	require.NotEqual(t, revision, decided[0], "the failed pass must not decide the new revision")
	f.destination.fwdFacts.Store(facts)
	require.Equal(t, revision, aigpRevision(), "retry without a new metric change")
	f.r.readvertiseAIGP()
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, 2)
	lastAnnounces(t, bodies, []byte{24, 10, 20, 0}, 111)
}

// TestAtomicPreparationFatalAndSuppressedIsNotSuccess preserves the public
// distinction between complete reflection suppression and a fatal destination.
func TestAtomicPreparationFatalAndSuppressedIsNotSuccess(t *testing.T) {
	f, id := atomicPreparationFault(t, false)
	suppressed, suppressedConn := atomicPreparationExtraPeer(t, f, 4)
	// Non-client iBGP pairs are truly suppressed. A filter reject instead
	// dispatches a source withdrawal, which is a useful forwarding receipt.
	f.source.settings.PeerAS = f.source.settings.LocalAS
	suppressed.settings.PeerAS = suppressed.settings.LocalAS
	f.source.refreshForwardFacts()
	suppressed.refreshForwardFacts()
	require.True(t, f.source.IsIBGP())
	require.True(t, suppressed.IsIBGP())
	require.False(t, f.source.Settings().RouteReflectorClient)
	require.False(t, suppressed.Settings().RouteReflectorClient)
	require.True(t, f.destination.IsEBGP(), "the recipient with unencodable removals remains eligible")
	sel, err := selector.Parse("*")
	require.NoError(t, err)
	err = (&reactorAPIAdapter{r: f.r}).ForwardUpdate(sel, id, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder"))
	require.ErrorIs(t, err, errNoEstablishedPeersToForwardTo)
	require.NotErrorIs(t, err, errAllDestinationsSuppressed)
	require.True(t, f.destination.currentSession().tearingDown.Load())
	require.False(t, suppressed.currentSession().tearingDown.Load(), "deliberate suppression does not make the unrelated session fatal")
	f.drain(t)
	require.Empty(t, suppressedConn.written())
}

func atomicPreparationExtraPeer(t *testing.T, f *aigpReplayFixture, last byte) (*Peer, *recordingConn) {
	t.Helper()
	peer, conn := newAnnouncePeer(t, netip.AddrFrom4([4]byte{198, 18, 235, last}).String())
	peer.settings.LocalAddress = f.destination.settings.LocalAddress
	peer.settings.GlobalLocalAS = f.destination.settings.GlobalLocalAS
	peer.settings.NextHopMode = NextHopUnchanged
	peer.settings.ImportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	peer.settings.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
	peer.settings.ProcessBindings = []ProcessBinding{{PluginName: "aigp-forwarder", SendAll: true}, rsRelaySendBinding()}
	negotiateForwardTestFamilies(peer, family.IPv4Unicast, family.IPv6Unicast,
		family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState})
	peer.session.negotiated.ExtendedMessageSend = true
	peer.session.writeMu.Lock()
	peer.session.writeBuf.Resize(peer.session.negotiated.ExtendedMessageSend)
	peer.session.writeMu.Unlock()
	peer.negotiated.Store(NewNegotiatedCapabilities(peer.session.negotiated))
	peer.sendCtx.Store(f.destination.sendContext())
	peer.sendCtxID, peer.recvCtxID = f.destination.sendCtxID, f.ctxID
	peer.session.sendCtxID = peer.sendCtxID
	peer.session.adjOut = &peer.adjOut
	peer.refreshForwardFacts()
	f.r.mu.Lock()
	f.r.peers[peer.Settings().PeerKey()] = peer
	f.r.mu.Unlock()
	f.r.fwdPool.registerOutgoingPool(fwdKey{peerAddr: peer.Settings().PeerKey()}, message.ExtMsgLen)
	return peer, conn
}

// TestAtomicPreparationPartitionedFanoutKeepsHealthyRecipients observes five
// useful recipient sockets after one peer's accepted removals fail. Validation
// creates two real partitions, so the result must retain the healthy receipts
// beyond its inline slots without counting the terminal recipient as success.
func TestAtomicPreparationPartitionedFanoutKeepsHealthyRecipients(t *testing.T) {
	f, id := atomicPreparationFault(t, false)
	update, present := f.r.recentUpdates.Get(id)
	require.True(t, present)
	plain, err := message.UnpackUpdate(update.WireUpdate.Payload())
	require.NoError(t, err)
	denied := []byte{24, 10, 45, 0}
	input := makeUpdateBody(nil, plain.PathAttributes, append(bytes.Clone(plain.NLRI), denied...))
	id = f.receive(t, input)
	ribevents.RegisterValidationLookup(func(route ribevents.ValidationRoute, _ uint64) bool {
		return route.Prefix != netip.MustParsePrefix("10.45.0.0/24")
	}, func(ribevents.ValidationRoute) bool { return false })
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	var peers []*Peer
	var connections []*recordingConn
	for i := byte(1); i <= 5; i++ {
		peer, conn := atomicPreparationExtraPeer(t, f, i)
		peers = append(peers, peer)
		connections = append(connections, conn)
	}
	all := append([]*Peer{f.destination}, peers...)
	exportLibraryInputAnswer(t, f.r, func(route rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
		if route.Peer != f.destination.addrString {
			return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
		}
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: atomicAdmissionMPBody(t, nil, []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2}, nil, true, 77)}
	}, all...)
	sel, err := selector.Parse("*")
	require.NoError(t, err)
	require.NoError(t, (&reactorAPIAdapter{r: f.r}).ForwardUpdate(sel, id, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder")),
		"five independently useful destinations must preserve partial success")
	forwardSocketBarrier(t, f.r)
	require.True(t, f.destination.currentSession().tearingDown.Load())
	for i, peer := range peers {
		require.False(t, peer.currentSession().tearingDown.Load())
		var legacy, native []byte
		for _, body := range aigpSocketBodies(t, connections[i]) {
			plain, err := message.UnpackUpdate(body)
			require.NoError(t, err)
			legacy = append(legacy, plain.NLRI...)
			reach, err := wireu.NewWireUpdate(body, peer.sendCtxID).MPReach()
			require.NoError(t, err)
			if reach != nil {
				native = append(native, reach.NLRIBytes()...)
			}
		}
		require.Equal(t, []byte{24, 10, 42, 0}, legacy)
		require.Equal(t, []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2}, native)
	}
}
