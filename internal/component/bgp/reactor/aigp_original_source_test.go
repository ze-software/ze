// Design: docs/architecture/route-selection.md -- original-source AIGP replay.
// Related: rfc7311_aigp_readvertise_test.go -- recording final writer and real receipt capture.
package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// Same-address inputs have different real PeerOwners and physical Sessions.
// Both actual egress builders reach the recording final writer. This question
// deliberately supplies original facts to the general builder: the separate
// public ForwardUpdate address lookup migration is not asserted here.
func TestAIGPReplayKeepsSameAddressOriginals(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "general", true: "native-rs"}[native], func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			f.source.settings.AIGPLinkMetric = 11
			sibling := aigpOwnerSibling(t, f, 2179, 29)
			first := f.receive(t, f.body(t, 100))
			aigpOwnerForward(t, f, first, native)
			lastAnnounces(t, aigpSocketBodies(t, f.conn), []byte{24, 10, 20, 0}, 111)
			second := f.receiveFrom(t, sibling, 200)
			aigpOwnerForward(t, f, second, native)
			lastAnnounces(t, aigpSocketBodies(t, f.conn), []byte{24, 10, 20, 0}, 229)
			f.r.aigp.mu.Lock()
			retained := len(f.r.aigp.routes)
			var selected *aigpAdvertisement
			for _, ad := range f.r.aigp.advertised {
				selected = ad
			}
			f.r.aigp.mu.Unlock()
			require.Equal(t, 2, retained, "one original must not overwrite its same-address sibling")
			require.NotNil(t, selected)
			require.Equal(t, sibling.sourceOwner, selected.route.key.source.owner)
			require.Equal(t, sibling.currentSession().connectionID, selected.route.key.source.connection)
			// The losing A withdrawal must not retire B's cost-replay obligation.
			withdrawal := []byte{0, 4, 24, 10, 20, 0, 0, 0}
			id := aigpFixtureReceive(t, f, f.source, f.source.currentSession(), withdrawal)
			aigpOwnerForward(t, f, id, native)
			sibling.settings.AIGPLinkMetric = 33
			sibling.refreshForwardFacts()
			f.metric(0) // A real Routing Table revision; direct zero uses the configured link.
			before := len(aigpSocketBodies(t, f.conn))
			f.r.readvertiseAIGP()
			forwardSocketBarrier(t, f.r)
			bodies := aigpSocketBodies(t, f.conn)
			require.Len(t, bodies, before+1)
			lastAnnounces(t, bodies, []byte{24, 10, 20, 0}, 233)
		})
	}
}

// An old exact physical cleanup and invalidation cannot erase the newer same
// logical owner's retained bytes or its already useful replay obligation.
func TestAIGPPhysicalCleanupKeepsSuccessor(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.metric(11)
	old := f.source.currentSession()
	oldID := f.receive(t, f.body(t, 100))
	aigpOwnerForward(t, f, oldID, false)
	oldReceipt := old.sourceReceiptID()
	replacement := aigpOwnerReconnect(t, f)
	currentID := f.receive(t, f.body(t, 200))
	aigpOwnerForward(t, f, currentID, false)
	require.NotEqual(t, old.connectionID, replacement.connectionID)
	require.NotEqual(t, oldReceipt, replacement.sourceReceiptID())
	f.r.aigp.mu.Lock()
	physicalRoutes := len(f.r.aigp.routes)
	f.r.aigp.mu.Unlock()
	require.Equal(t, 2, physicalRoutes, "one logical owner retains distinct original physical sources")
	f.r.forgetClosedPeer(f.source, old)
	previous := wireu.NewWireUpdate([]byte{0, 4, 24, 10, 20, 0, 0, 0}, f.ctxID)
	previous.SetSourceID(f.source.SourceID())
	previous.SetSourceOwner(f.source.sourceOwner)
	previous.SetSourceReceipt(oldReceipt)
	f.r.invalidateAIGPReceived(f.source.Settings().Address, previous)
	f.metric(7)
	before := len(aigpSocketBodies(t, f.conn))
	f.r.readvertiseAIGP()
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, before+1)
	lastAnnounces(t, bodies, []byte{24, 10, 20, 0}, 207)
	old.closeSourceReceipt()
}

// Real queued replay is held before the actual worker. A successor remains
// useful, and releasing the old item cannot borrow that successor's facts.
func TestAIGPHeldReplayCannotBorrowSuccessor(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	f := newAIGPReplayFixture(t, func(items []fwdItem) {
		for i := range items {
			if items[i].aigpReplay != nil {
				select {
				case entered <- struct{}{}:
				default:
				}
				<-release
				return
			}
		}
	})
	// The registered cleanup MUST unblock the worker before the fixture joins it.
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	f.metric(11)
	aigpOwnerForward(t, f, f.receive(t, f.body(t, 100)), false)
	f.metric(7)
	f.r.readvertiseAIGP()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("actual replay did not reach held worker")
	}
	old := f.source.currentSession()
	aigpOwnerReconnect(t, f)
	fresh := f.receive(t, f.body(t, 200))
	update, ok := f.r.recentUpdates.Get(fresh)
	require.True(t, ok)
	source, err := aigpOriginalSource(update.SourcePeerIP, update.WireUpdate)
	require.NoError(t, err)
	source.info.sender = plugin.ProcessSender("aigp-forwarder")
	require.NoError(t, (&reactorAPIAdapter{r: f.r}).forwardUpdateCore(update, fresh, []*Peer{f.destination}, source.info))
	close(release)
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, 2, "old replay must not add a third UPDATE")
	lastAnnounces(t, bodies, []byte{24, 10, 20, 0}, 207)
	f.r.forgetClosedPeer(f.source, old)
	old.closeSourceReceipt()
}

// A closed original receipt cannot be reconstructed with an established sibling
// just because its address matches. The sibling's independent route still moves.
func TestAIGPReleasedOriginalRefusesReplayKeepsSibling(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.source.settings.AIGPLinkMetric = 11
	sibling := aigpOwnerSibling(t, f, 2179, 29)
	aigpOwnerForward(t, f, f.receive(t, f.body(t, 100)), false)
	body := f.body(t, 200)
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	prefix := sections.NLRI(body)
	require.Len(t, prefix, 4)
	prefix[2] = 21
	siblingID := aigpFixtureReceive(t, f, sibling, sibling.currentSession(), body)
	aigpOwnerForward(t, f, siblingID, false)
	f.source.currentSession().closeSourceReceipt()
	sibling.settings.AIGPLinkMetric = 33
	sibling.refreshForwardFacts()
	f.metric(0)
	before := len(aigpSocketBodies(t, f.conn))
	f.r.readvertiseAIGP()
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, before+1, "unknown original must not borrow its live sibling")
	lastAnnounces(t, bodies, []byte{24, 10, 21, 0}, 233)
}

// Receipt, owner, context and physical mismatches are real refusals. A valid
// exact original retains its configured name/AS/context.
func TestAIGPOriginalSourceRejectsMismatchedReceipt(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.source.settings.Name = "aigp-original"
	f.source.refreshForwardFacts()
	sibling := aigpOwnerSibling(t, f, 2179, 29)
	id := f.receive(t, f.body(t, 100))
	update, ok := f.r.recentUpdates.Get(id)
	require.True(t, ok)
	source, err := aigpOriginalSource(f.source.Settings().Address, update.WireUpdate)
	require.NoError(t, err)
	require.Same(t, f.source, source.info.peer)
	require.Equal(t, f.source.sourceOwner, source.info.filterInfo.PeerOwner)
	require.Equal(t, f.source.currentSession().connectionID, source.info.filterInfo.ConnectionID)
	require.Equal(t, f.ctxID, source.ctxID)
	require.NotEqual(t, sibling.sourceOwner, source.info.filterInfo.PeerOwner)
	require.Equal(t, "aigp-original", source.info.filterInfo.Name)
	require.Equal(t, f.source.Settings().PeerAS, source.info.filterInfo.PeerAS)
	require.Equal(t, f.source.Settings().IsIBGP(), source.info.isIBGP)
	f.r.aigp.mu.Lock()
	var retained *aigpRoute
	for _, route := range f.r.aigp.routes {
		retained = route
	}
	f.r.aigp.mu.Unlock()
	require.NotNil(t, retained)
	require.Equal(t, id, retained.wire.MessageID())
	require.Equal(t, update.SourcePolicyEpoch, retained.sourcePolicyEpoch)
	originalAS := source.info.filterInfo.PeerAS
	f.source.settings.Name = "later-source-name"
	f.source.settings.PeerAS = 65123
	f.source.refreshForwardFacts()
	captured, err := aigpOriginalSource(f.source.Settings().Address, update.WireUpdate)
	require.NoError(t, err)
	require.Equal(t, "aigp-original", captured.info.filterInfo.Name)
	require.Equal(t, originalAS, captured.info.filterInfo.PeerAS, "current Peer settings cannot rewrite original export facts")
	for _, bad := range []uint64{0, sibling.sourceOwner} {
		clone := wireu.NewWireUpdate(bytes.Clone(update.WireUpdate.Payload()), f.ctxID)
		clone.SetSourceID(f.source.SourceID())
		clone.SetSourceReceipt(update.SourceReceipt)
		clone.SetSourceOwner(bad)
		_, err := aigpOriginalSource(f.source.Settings().Address, clone)
		require.Error(t, err)
	}
	differentContext, contextErr := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true}))
	require.NoError(t, contextErr)
	wrongContext := wireu.NewWireUpdate(bytes.Clone(update.WireUpdate.Payload()), differentContext)
	wrongContext.SetSourceID(f.source.SourceID())
	wrongContext.SetSourceReceipt(update.SourceReceipt)
	wrongContext.SetSourceOwner(f.source.sourceOwner)
	_, err = aigpOriginalSource(f.source.Settings().Address, wrongContext)
	require.Error(t, err)
	wrongSourceID := wireu.NewWireUpdate(bytes.Clone(update.WireUpdate.Payload()), f.ctxID)
	wrongSourceID.SetSourceID(f.source.SourceID() + 1)
	wrongSourceID.SetSourceReceipt(update.SourceReceipt)
	wrongSourceID.SetSourceOwner(f.source.sourceOwner)
	_, err = aigpOriginalSource(f.source.Settings().Address, wrongSourceID)
	require.Error(t, err)
	_, err = aigpOriginalSource(netip.MustParseAddr("192.0.2.99"), update.WireUpdate)
	require.Error(t, err)
}

// Legal received ADD-PATH zero remains a distinct path through actual raw and
// converted writes. A sibling's path zero withdrawal must not erase the original.
func TestAIGPOriginalSourceAddPathZeroRawAndConverted(t *testing.T) {
	for _, addOutput := range []bool{false, true} {
		t.Run(map[bool]string{false: "converted", true: "raw"}[addOutput], func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			context := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
			contextID, err := bgpctx.Registry.Register(context)
			require.NoError(t, err)
			f.ctxID = contextID
			f.source.recvCtxID = contextID
			sibling := aigpOwnerSibling(t, f, 2179, 29)
			sibling.recvCtxID = contextID
			if addOutput {
				f.destination.sendCtx.Store(context)
				f.destination.sendCtxID = contextID
				f.destination.session.sendCtxID = contextID
				f.destination.refreshForwardFacts()
			}
			f.metric(7)
			pathBody := func(metric uint64) []byte {
				body := f.body(t, metric)
				sections, err := wire.ParseUpdateSections(body)
				require.NoError(t, err)
				nlri := binary.BigEndian.AppendUint32(nil, 0)
				return makeUpdateBody(nil, sections.Attrs(body), append(nlri, sections.NLRI(body)...))
			}
			first := aigpFixtureReceive(t, f, f.source, f.source.currentSession(), pathBody(100))
			aigpOwnerForward(t, f, first, false)
			second := aigpFixtureReceive(t, f, sibling, sibling.currentSession(), pathBody(200))
			aigpOwnerForward(t, f, second, false)
			initial := aigpSocketBodies(t, f.conn)
			require.Len(t, initial, 2)
			if addOutput {
				var ids [2]uint32
				for i, body := range initial {
					sections, err := wire.ParseUpdateSections(body)
					require.NoError(t, err)
					ids[i] = binary.BigEndian.Uint32(sections.NLRI(body)[:4])
				}
				require.NotEqual(t, ids[0], ids[1], "same-address owners with received path zero need different output paths")
			}
			withdrawal := binary.BigEndian.AppendUint32(nil, 0)
			withdrawal = append(withdrawal, 24, 10, 20, 0)
			id := aigpFixtureReceive(t, f, f.source, f.source.currentSession(), makeUpdateBody(withdrawal, nil, nil))
			aigpOwnerForward(t, f, id, false)
			before := len(aigpSocketBodies(t, f.conn))
			f.metric(13)
			f.r.readvertiseAIGP()
			forwardSocketBarrier(t, f.r)
			bodies := aigpSocketBodies(t, f.conn)
			require.Len(t, bodies, before+1, "one original path zero withdrawal must preserve its sibling")
			metric, present := aigpReceivedMetric(t, bodies[before])
			require.True(t, present)
			require.Equal(t, uint64(213), metric)
			sections, err := wire.ParseUpdateSections(bodies[before])
			require.NoError(t, err)
			nlri := sections.NLRI(bodies[before])
			if addOutput {
				initialSections, err := wire.ParseUpdateSections(initial[1])
				require.NoError(t, err)
				require.Equal(t, binary.BigEndian.Uint32(initialSections.NLRI(initial[1])[:4]), binary.BigEndian.Uint32(nlri[:4]))
				nlri = nlri[4:]
			}
			require.Equal(t, []byte{24, 10, 20, 0}, nlri)
		})
	}
}

// The new source has the normal Session birth and established FSM, with no TCP
// listener: only the destination recording connection writes in these questions.
func aigpOwnerReconnect(t *testing.T, f *aigpReplayFixture) *Session {
	t.Helper()
	session := sourceSessionFixture(t, f.source)
	for _, event := range []fsm.Event{fsm.EventManualStart, fsm.EventTCPConnectionConfirmed, fsm.EventBGPOpen, fsm.EventKeepaliveMsg} {
		require.NoError(t, session.fsm.Event(event))
	}
	session.setRecvCtxID(f.ctxID)
	f.source.recvCtxID = f.ctxID
	return session
}

func aigpOwnerSibling(t *testing.T, f *aigpReplayFixture, port uint16, metric uint64) *Peer {
	t.Helper()
	firstKey := f.source.Settings().PeerKey()
	peer := f.addSource(t, f.source.Settings().Address.String())
	peer.settings.Port = port
	peer.settings.Name = "aigp-sibling"
	peer.settings.AIGPLinkMetric = metric
	peer.refreshForwardFacts()
	f.r.mu.Lock()
	f.r.peers[firstKey] = f.source
	f.r.peers[peer.Settings().PeerKey()] = peer
	f.r.mu.Unlock()
	require.NotEqual(t, f.source.sourceOwner, peer.sourceOwner)
	require.Equal(t, f.source.SourceID(), peer.SourceID(), "real registry aliases the shared address")
	return peer
}

func aigpOwnerForward(t *testing.T, f *aigpReplayFixture, id uint64, native bool) {
	t.Helper()
	update, ok := f.r.recentUpdates.Get(id)
	require.True(t, ok)
	source, err := aigpOriginalSource(update.SourcePeerIP, update.WireUpdate)
	require.NoError(t, err)
	if native {
		_, sent, _ := reactorForwardRSSection(f.r, update, update.WireUpdate, []*Peer{f.destination}, id, source.addr, source.info.peer)
		require.Equal(t, 1, sent)
	} else {
		source.info.sender = plugin.ProcessSender("aigp-forwarder")
		require.NoError(t, (&reactorAPIAdapter{r: f.r}).forwardUpdateCore(update, id, []*Peer{f.destination}, source.info))
	}
	forwardSocketBarrier(t, f.r)
}
